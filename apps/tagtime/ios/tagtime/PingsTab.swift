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
    @State private var initializedAt: Date?
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
            .toolbar {
                if let initializedAt {
                    ToolbarItem(placement: .bottomBar) {
                        Text("Tracking since \(initializedAt, format: .dateTime.month(.wide).day().year().hour().minute())")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }
                }
            }
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
        guard nodeManager.isRunning else { return }
        guard let url = URL(string: "\(nodeManager.baseURL)/pings") else { return }
        guard let (data, _) = try? await URLSession.shared.data(from: url) else { return }

        struct PingsPayload: Codable {
            let pending: [Ping]?
            let recent: [Ping]?
            let initialized_at: Int64?
        }
        guard let payload = try? JSONDecoder().decode(PingsPayload.self, from: data) else { return }

        await MainActor.run {
            pending = payload.pending ?? []
            recent = payload.recent ?? []
            selectedTimestamps = Set(pending.map(\.timestamp))
            if let ts = payload.initialized_at, ts > 0 {
                initializedAt = Date(timeIntervalSince1970: TimeInterval(ts))
            }
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
