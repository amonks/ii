import SwiftUI

struct Ping: Codable, Identifiable {
    let timestamp: Int64
    let blurb: String
    let node_id: String
    let updated_at: Int64
    let synced_at: Int64

    var id: Int64 { timestamp }

    var date: Date {
        Date(timeIntervalSince1970: TimeInterval(timestamp))
    }
}

struct PingsTab: View {
    @EnvironmentObject var nodeManager: NodeManager
    @State private var pending: [Ping] = []
    @State private var recent: [Ping] = []
    @State private var batchBlurb = ""
    @State private var selectedTimestamps: Set<Int64> = []
    @State private var answerTexts: [Int64: String] = [:]

    var body: some View {
        NavigationView {
            List {
                if !pending.isEmpty {
                    Section("Batch Set") {
                        HStack {
                            TextField("#sleeping", text: $batchBlurb)
                                .textFieldStyle(.roundedBorder)
                            Button("Set All") {
                                batchAnswer()
                            }
                            .disabled(batchBlurb.isEmpty || selectedTimestamps.isEmpty)
                        }
                    }

                    Section("Pending (\(pending.count))") {
                        ForEach(pending) { ping in
                            VStack(alignment: .leading, spacing: 4) {
                                HStack {
                                    Toggle(isOn: Binding(
                                        get: { selectedTimestamps.contains(ping.timestamp) },
                                        set: { isOn in
                                            if isOn {
                                                selectedTimestamps.insert(ping.timestamp)
                                            } else {
                                                selectedTimestamps.remove(ping.timestamp)
                                            }
                                        }
                                    )) {
                                        Text(ping.date, style: .date)
                                        Text(ping.date, style: .time)
                                    }
                                    .toggleStyle(.checkmark)
                                }

                                HStack {
                                    TextField("What were you doing? #tag", text: binding(for: ping.timestamp))
                                        .textFieldStyle(.roundedBorder)
                                    Button("Save") {
                                        answer(timestamp: ping.timestamp)
                                    }
                                    .disabled((answerTexts[ping.timestamp] ?? "").isEmpty)
                                }
                            }
                            .padding(.vertical, 4)
                        }
                    }
                }

                if !recent.isEmpty {
                    Section("Recent") {
                        ForEach(recent) { ping in
                            VStack(alignment: .leading) {
                                Text(ping.date, style: .relative)
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                                Text(ping.blurb)
                            }
                        }
                    }
                }
            }
            .navigationTitle("TagTime")
            .refreshable { await refresh() }
            .task { await refresh() }
        }
    }

    private func binding(for timestamp: Int64) -> Binding<String> {
        Binding(
            get: { answerTexts[timestamp, default: ""] },
            set: { answerTexts[timestamp] = $0 }
        )
    }

    private func refresh() async {
        // Fetch pending and recent pings from local node.
        guard nodeManager.isRunning else { return }
        let base = nodeManager.baseURL

        // We parse the HTML response is impractical; instead use the sync/pull endpoint
        // to get raw JSON data and filter locally.
        guard let url = URL(string: "\(base)/sync/pull?since=0") else { return }
        guard let (data, _) = try? await URLSession.shared.data(from: url) else { return }

        struct SyncPayload: Codable {
            let pings: [Ping]?
        }
        guard let payload = try? JSONDecoder().decode(SyncPayload.self, from: data) else { return }
        let allPings = (payload.pings ?? []).sorted { $0.timestamp > $1.timestamp }

        await MainActor.run {
            pending = allPings.filter { $0.blurb.isEmpty && Date(timeIntervalSince1970: TimeInterval($0.timestamp)) <= Date() }
            recent = Array(allPings.filter { !$0.blurb.isEmpty }.prefix(20))
            selectedTimestamps = Set(pending.map(\.timestamp))
        }
    }

    private func answer(timestamp: Int64) {
        guard let blurb = answerTexts[timestamp], !blurb.isEmpty else { return }
        post(path: "/answer", body: "timestamp=\(timestamp)&blurb=\(blurb.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? blurb)")
        answerTexts[timestamp] = nil
        Task { await refresh() }
    }

    private func batchAnswer() {
        let timestamps = selectedTimestamps.map { "timestamps=\($0)" }.joined(separator: "&")
        let blurb = batchBlurb.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? batchBlurb
        post(path: "/batch-answer", body: "\(timestamps)&blurb=\(blurb)")
        batchBlurb = ""
        Task { await refresh() }
    }

    private func post(path: String, body: String) {
        guard let url = URL(string: "\(nodeManager.baseURL)\(path)") else { return }
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/x-www-form-urlencoded", forHTTPHeaderField: "Content-Type")
        request.httpBody = body.data(using: .utf8)
        URLSession.shared.dataTask(with: request) { _, _, _ in }.resume()
    }
}

extension ToggleStyle where Self == CheckmarkToggleStyle {
    static var checkmark: CheckmarkToggleStyle { CheckmarkToggleStyle() }
}

struct CheckmarkToggleStyle: ToggleStyle {
    func makeBody(configuration: Configuration) -> some View {
        Button(action: { configuration.isOn.toggle() }) {
            HStack {
                Image(systemName: configuration.isOn ? "checkmark.circle.fill" : "circle")
                    .foregroundStyle(configuration.isOn ? .blue : .gray)
                configuration.label
            }
        }
        .buttonStyle(.plain)
    }
}
