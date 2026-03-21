//
//  ContentView.swift
//  Breadcrumbs
//
//  Created by Andrew Monks on 3/19/26.
//

import SwiftUI
import SwiftData

struct ContentView: View {
    @Bindable var logger: LocationLogger
    @Environment(\.modelContext) private var modelContext

    @State private var latest: LocationRecord?
    @State private var count: Int = 0

    var body: some View {
        List {
            Section {
                Text("\(count) records")
                    .font(.system(.body, design: .monospaced))
            }

            Section {
                Toggle("Logging", isOn: $logger.isEnabled)
            }

            if let loc = latest {
                Section("Latest Record") {
                    row("Timestamp", loc.timestamp.formatted(.dateTime.hour().minute().second().secondFraction(.fractional(3))))
                    row("Latitude", String(format: "%.6f", loc.latitude))
                    row("Longitude", String(format: "%.6f", loc.longitude))
                    row("Altitude (MSL)", String(format: "%.1f m", loc.altitude))
                    row("Altitude (WGS84)", String(format: "%.1f m", loc.ellipsoidalAltitude))
                    row("H Accuracy", String(format: "%.1f m", loc.horizontalAccuracy))
                    row("V Accuracy", String(format: "%.1f m", loc.verticalAccuracy))
                    row("Speed", String(format: "%.1f m/s", loc.speed))
                    row("Speed Accuracy", String(format: "%.1f m/s", loc.speedAccuracy))
                    row("Course", String(format: "%.1f°", loc.course))
                    row("Course Accuracy", String(format: "%.1f°", loc.courseAccuracy))
                    if let floor = loc.floor {
                        row("Floor", "\(floor)")
                    }
                    row("Simulated", loc.isSimulatedBySoftware ? "Yes" : "No")
                    row("Accessory", loc.isProducedByAccessory ? "Yes" : "No")
                }
            } else {
                Section {
                    Text("Waiting for location...")
                        .foregroundStyle(.secondary)
                }
            }
        }
        .navigationTitle("Breadcrumbs")
        .onChange(of: logger.storeCount) {
            refresh()
        }
        .onAppear {
            refresh()
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

    private func refresh() {
        var descriptor = FetchDescriptor<LocationRecord>(
            sortBy: [SortDescriptor(\.timestamp, order: .reverse)]
        )
        descriptor.fetchLimit = 1
        latest = try? modelContext.fetch(descriptor).first
        count = (try? modelContext.fetchCount(FetchDescriptor<LocationRecord>())) ?? 0
    }
}
