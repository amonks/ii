//
//  ContentView.swift
//  maplog
//
//  Created by Andrew Monks on 3/19/26.
//

import SwiftUI
import SwiftData
import SwiftProtobuf

struct ContentView: View {
    @Bindable var logger: LocationLogger
    let nodePort: Int
    let clientID: String

    @AppStorage("detail") private var detail: Double = 5

    var body: some View {
        TabView {
            Tab("Map", systemImage: "map") {
                MapTab(nodePort: nodePort, clientID: clientID, detail: detail)
            }
            Tab("Status", systemImage: "info.circle") {
                StatusTab(logger: logger, nodePort: nodePort, clientID: clientID, detail: $detail)
            }
        }
    }
}

struct StatusTab: View {
    @Bindable var logger: LocationLogger
    let nodePort: Int
    let clientID: String
    @Binding var detail: Double

    @Environment(\.modelContext) private var modelContext
    @State private var legacyCount: Int = 0
    @State private var migrating: Bool = false
    @State private var migrated: Int = 0

    @State private var nodeCount: Int64 = 0
    @State private var latestTimestamp: Date? = nil
    @State private var latestLat: Double? = nil
    @State private var latestLon: Double? = nil
    @State private var statsError: String? = nil
    @State private var flushing: Bool = false

    var body: some View {
        NavigationStack {
            List {
                Section("Node") {
                    row("Port", "\(nodePort)")
                    if let err = statsError {
                        row("Status", "Error: \(err)")
                    } else {
                        row("Status", "Running")
                    }
                }

                Section("Database") {
                    row("Points stored", "\(nodeCount)")
                    if let ts = latestTimestamp {
                        row("Latest at", ts.formatted(.dateTime.month().day().hour().minute().second()))
                    }
                    if let lat = latestLat, let lon = latestLon {
                        row("Latest pos", String(format: "%.5f, %.5f", lat, lon))
                    }
                }

                Section("Session") {
                    row("Points ingested", "\(logger.storeCount)")
                    row("Last watermark", "\(logger.lastWatermark)")
                }

                Section("Map") {
                    VStack(alignment: .leading) {
                        Text("Detail: \(Int(detail))")
                        Slider(value: $detail, in: 0...10, step: 1)
                    }
                }

                Section {
                    Toggle("Logging", isOn: $logger.isEnabled)
                    Button("Sync Now") {
                        Task { await flush() }
                    }
                    .disabled(flushing)
                }

                if legacyCount > 0 {
                    Section("Legacy Data") {
                        row("SwiftData records", "\(legacyCount)")
                        if migrating {
                            ProgressView("Migrating... \(migrated)/\(legacyCount)")
                        } else {
                            Button("Migrate to node") {
                                Task { await migrate() }
                            }
                            Button("Delete legacy data", role: .destructive) {
                                deleteLegacy()
                            }
                        }
                    }
                }
            }
            .navigationTitle("maplog")
        }
        .onAppear {
            refreshLegacyCount()
            Task { await refreshStats() }
        }
        .task {
            // Poll stats every 5 seconds.
            while !Task.isCancelled {
                try? await Task.sleep(for: .seconds(5))
                await refreshStats()
            }
        }
    }

    private func row(_ label: String, _ value: String) -> some View {
        HStack {
            Text(label)
                .foregroundStyle(.secondary)
            Spacer()
            Text(value)
                .font(.system(.body, design: .monospaced))
        }
    }

    private func refreshStats() async {
        let url = URL(string: "http://127.0.0.1:\(nodePort)/stats")!
        do {
            let (data, _) = try await URLSession.shared.data(from: url)
            let resp = try Breadcrumbs_StatsResponse(serializedBytes: data)
            nodeCount = resp.count
            if resp.hasLatestPoint {
                let ns = resp.latestPoint.timestamp
                latestTimestamp = Date(timeIntervalSince1970: Double(ns) / 1_000_000_000)
                latestLat = resp.latestPoint.latitude
                latestLon = resp.latestPoint.longitude
            } else {
                latestTimestamp = nil
                latestLat = nil
                latestLon = nil
            }
            statsError = nil
        } catch {
            statsError = error.localizedDescription
        }
    }

    private func refreshLegacyCount() {
        legacyCount = (try? modelContext.fetchCount(FetchDescriptor<LocationRecord>())) ?? 0
    }

    private func migrate() async {
        migrating = true
        migrated = 0

        let batchSize = 100
        var offset = 0

        while true {
            var descriptor = FetchDescriptor<LocationRecord>(
                sortBy: [SortDescriptor(\.timestamp)]
            )
            descriptor.fetchOffset = offset
            descriptor.fetchLimit = batchSize

            guard let records = try? modelContext.fetch(descriptor), !records.isEmpty else {
                break
            }

            var track = Breadcrumbs_Track()
            track.points = records.map { rec in
                var p = Breadcrumbs_Point()
                p.timestamp = Int64(rec.timestamp.timeIntervalSince1970 * 1_000_000_000)
                p.latitude = rec.latitude
                p.longitude = rec.longitude
                p.altitude = rec.altitude
                p.ellipsoidalAltitude = rec.ellipsoidalAltitude
                p.horizontalAccuracy = rec.horizontalAccuracy
                p.verticalAccuracy = rec.verticalAccuracy
                p.speed = rec.speed
                p.speedAccuracy = rec.speedAccuracy
                p.course = rec.course
                p.courseAccuracy = rec.courseAccuracy
                p.floor = Int32(rec.floor ?? 0)
                p.isSimulated = rec.isSimulatedBySoftware
                p.isFromAccessory = rec.isProducedByAccessory
                return p
            }

            guard let body = try? track.serializedData() else { break }

            var request = URLRequest(url: URL(string: "http://127.0.0.1:\(nodePort)/ingest")!)
            request.httpMethod = "POST"
            request.httpBody = body
            request.setValue("application/protobuf", forHTTPHeaderField: "Content-Type")

            do {
                let (_, response) = try await URLSession.shared.data(for: request)
                guard (response as? HTTPURLResponse)?.statusCode == 200 else { break }
            } catch {
                break
            }

            offset += records.count
            migrated = offset
        }

        migrating = false
        refreshLegacyCount()
    }

    private func deleteLegacy() {
        try? modelContext.delete(model: LocationRecord.self)
        try? modelContext.save()
        refreshLegacyCount()
    }

    private func flush() async {
        flushing = true
        defer { flushing = false }
        var request = URLRequest(url: URL(string: "http://127.0.0.1:\(nodePort)/flush")!)
        request.httpMethod = "POST"
        _ = try? await URLSession.shared.data(for: request)
    }
}
