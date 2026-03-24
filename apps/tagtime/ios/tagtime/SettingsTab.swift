import SwiftUI

struct PeriodChange: Codable, Identifiable {
    let timestamp: Int64
    let seed: UInt64
    let period_secs: Int64

    var id: Int64 { timestamp }

    var date: Date {
        Date(timeIntervalSince1970: TimeInterval(timestamp))
    }

    var periodMinutes: Int64 { period_secs / 60 }
}

struct NextPingResponse: Codable {
    let timestamp: Int64

    var date: Date {
        Date(timeIntervalSince1970: TimeInterval(timestamp))
    }
}

struct SyncStatusResponse: Codable {
    let has_upstream: Bool
    let upstream: String?
    let unsynced_count: Int
    let pull_watermark: String
    let last_push_at: String
    let last_pull_at: String
}

struct SettingsTab: View {
    @EnvironmentObject var nodeManager: NodeManager
    @State private var changes: [PeriodChange] = []
    @State private var periodInput = ""
    @State private var nextPing: NextPingResponse?
    @State private var syncStatus: SyncStatusResponse?
    @State private var isSyncing = false
    @FocusState private var periodFieldFocused: Bool

    private var current: PeriodChange? { changes.last }

    var body: some View {
        NavigationView {
            List {
                Section("Next Ping") {
                    if let nextPing {
                        HStack {
                            Text(nextPing.date, style: .relative)
                            Spacer()
                            Text(nextPing.date, style: .time)
                                .foregroundStyle(.secondary)
                        }
                    } else {
                        Text("Loading...")
                            .foregroundStyle(.secondary)
                    }
                }

                Section("Current Period") {
                    if let current {
                        Text("\(current.periodMinutes) minutes")
                    } else {
                        Text("Loading...")
                            .foregroundStyle(.secondary)
                    }
                }

                Section("Change Period") {
                    HStack {
                        TextField("Minutes", text: $periodInput)
                            .textFieldStyle(.roundedBorder)
                            .keyboardType(.numberPad)
                            .focused($periodFieldFocused)
                        Button("Save") {
                            savePeriod()
                        }
                        .disabled(periodInput.isEmpty || Int(periodInput) == nil || Int(periodInput)! < 1)
                    }
                }

                Section("Sync") {
                    if let syncStatus {
                        if syncStatus.has_upstream {
                            HStack {
                                Text("Upstream")
                                Spacer()
                                Text(syncStatus.upstream ?? "")
                                    .foregroundStyle(.secondary)
                            }
                            HStack {
                                Text("Unsynced pings")
                                Spacer()
                                Text("\(syncStatus.unsynced_count)")
                                    .foregroundStyle(syncStatus.unsynced_count > 0 ? .orange : .secondary)
                            }
                            HStack {
                                Text("Last pushed")
                                Spacer()
                                Text(formatSyncTimestamp(syncStatus.last_push_at))
                                    .foregroundStyle(.secondary)
                            }
                            HStack {
                                Text("Last fetched")
                                Spacer()
                                Text(formatSyncTimestamp(syncStatus.last_pull_at))
                                    .foregroundStyle(.secondary)
                            }
                            Button {
                                triggerSync()
                            } label: {
                                HStack {
                                    Spacer()
                                    if isSyncing {
                                        ProgressView()
                                            .controlSize(.small)
                                        Text("Syncing...")
                                    } else {
                                        Text("Sync Now")
                                    }
                                    Spacer()
                                }
                            }
                            .disabled(isSyncing)
                        } else {
                            Text("No upstream configured")
                                .foregroundStyle(.secondary)
                        }
                    } else {
                        Text("Loading...")
                            .foregroundStyle(.secondary)
                    }
                }

                if changes.count > 1 {
                    Section("History") {
                        ForEach(changes.reversed().dropFirst()) { change in
                            HStack {
                                if change.timestamp == 0 {
                                    Text("Initial")
                                        .foregroundStyle(.secondary)
                                } else {
                                    Text(change.date, style: .date)
                                    Text(change.date, style: .time)
                                }
                                Spacer()
                                Text("\(change.periodMinutes) min")
                                    .foregroundStyle(.secondary)
                            }
                        }
                    }
                }
            }
            .navigationTitle("Settings")
            .task { await refresh() }
            .refreshable { await refresh() }
        }
    }

    private func refresh() async {
        guard nodeManager.isRunning else { return }

        // Fetch period changes, next ping, and sync status concurrently.
        async let changesResult: [PeriodChange]? = {
            guard let url = URL(string: "\(nodeManager.baseURL)/sync/period-changes"),
                  let (data, _) = try? await URLSession.shared.data(from: url)
            else { return nil }
            return try? JSONDecoder().decode([PeriodChange].self, from: data)
        }()

        async let nextPingResult: NextPingResponse? = {
            guard let url = URL(string: "\(nodeManager.baseURL)/next-ping"),
                  let (data, _) = try? await URLSession.shared.data(from: url)
            else { return nil }
            return try? JSONDecoder().decode(NextPingResponse.self, from: data)
        }()

        async let syncStatusResult: SyncStatusResponse? = {
            guard let url = URL(string: "\(nodeManager.baseURL)/sync/status"),
                  let (data, _) = try? await URLSession.shared.data(from: url)
            else { return nil }
            return try? JSONDecoder().decode(SyncStatusResponse.self, from: data)
        }()

        let (c, np, ss) = await (changesResult, nextPingResult, syncStatusResult)

        await MainActor.run {
            if let c { changes = c }
            if let np { nextPing = np }
            if let ss { syncStatus = ss }
        }
    }

    private func formatSyncTimestamp(_ s: String) -> String {
        guard let ts = Int64(s), ts > 0 else { return "never" }
        let date = Date(timeIntervalSince1970: TimeInterval(ts))
        let formatter = RelativeDateTimeFormatter()
        formatter.unitsStyle = .short
        return formatter.localizedString(for: date, relativeTo: Date())
    }

    private func triggerSync() {
        guard let url = URL(string: "\(nodeManager.baseURL)/sync/now") else { return }
        isSyncing = true
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        Task {
            _ = try? await URLSession.shared.data(for: request)
            await refresh()
            await MainActor.run { isSyncing = false }
        }
    }

    private func savePeriod() {
        guard let minutes = Int(periodInput), minutes >= 1 else { return }
        guard let url = URL(string: "\(nodeManager.baseURL)/settings/period") else { return }

        periodInput = ""
        periodFieldFocused = false

        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/x-www-form-urlencoded", forHTTPHeaderField: "Content-Type")
        request.httpBody = "period_minutes=\(minutes)".data(using: .utf8)

        Task {
            _ = try? await URLSession.shared.data(for: request)
            await refresh()
        }
    }
}
