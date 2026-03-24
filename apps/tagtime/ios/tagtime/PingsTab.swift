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
    @State private var editingTimestamp: Int64?
    @State private var editTexts: [Int64: String] = [:]
    @State private var allTags: [String] = []

    var body: some View {
        NavigationView {
            List {
                if !pending.isEmpty {
                    Section("Batch Set") {
                        TagTextField(
                            placeholder: "#sleeping",
                            text: $batchBlurb,
                            allTags: allTags
                        )
                        HStack {
                            Spacer()
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

                                TagTextField(
                                    placeholder: "What were you doing? #tag",
                                    text: binding(for: ping.timestamp),
                                    allTags: allTags
                                )
                                HStack {
                                    Spacer()
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
                            VStack(alignment: .leading, spacing: 4) {
                                Text(ping.date, style: .relative)
                                    .font(.caption)
                                    .foregroundStyle(.secondary)
                                if editingTimestamp == ping.timestamp {
                                    TagTextField(
                                        placeholder: "",
                                        text: editBinding(for: ping.timestamp),
                                        allTags: allTags
                                    )
                                    HStack {
                                        Spacer()
                                        Button("Save") {
                                            saveEdit(timestamp: ping.timestamp)
                                        }
                                        .disabled((editTexts[ping.timestamp] ?? "").isEmpty)
                                        Button("Cancel") {
                                            editingTimestamp = nil
                                        }
                                        .foregroundStyle(.secondary)
                                    }
                                } else {
                                    Text(ping.blurb)
                                        .onTapGesture {
                                            editTexts[ping.timestamp] = ping.blurb
                                            editingTimestamp = ping.timestamp
                                        }
                                }
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

    private func editBinding(for timestamp: Int64) -> Binding<String> {
        Binding(
            get: { editTexts[timestamp, default: ""] },
            set: { editTexts[timestamp] = $0 }
        )
    }

    private func refresh() async {
        guard nodeManager.isRunning else { return }

        // Fetch pings and tags in parallel.
        async let pingsResult = fetchPings()
        async let tagsResult = fetchTags()

        let (pingsData, tagsData) = await (pingsResult, tagsResult)

        await MainActor.run {
            if let pingsData {
                pending = pingsData.pending ?? []
                recent = pingsData.recent ?? []
                selectedTimestamps = Set(pending.map(\.timestamp))
                if let ts = pingsData.initialized_at, ts > 0 {
                    initializedAt = Date(timeIntervalSince1970: TimeInterval(ts))
                }
            }
            if let tagsData {
                allTags = tagsData
            }
        }
    }

    private struct PingsPayload: Codable {
        let pending: [Ping]?
        let recent: [Ping]?
        let initialized_at: Int64?
    }

    private func fetchPings() async -> PingsPayload? {
        guard let url = URL(string: "\(nodeManager.baseURL)/pings") else { return nil }
        guard let (data, _) = try? await URLSession.shared.data(from: url) else { return nil }
        return try? JSONDecoder().decode(PingsPayload.self, from: data)
    }

    private func fetchTags() async -> [String]? {
        guard let url = URL(string: "\(nodeManager.baseURL)/tags") else { return nil }
        guard let (data, _) = try? await URLSession.shared.data(from: url) else { return nil }
        return try? JSONDecoder().decode([String].self, from: data)
    }

    private func saveEdit(timestamp: Int64) {
        guard let blurb = editTexts[timestamp], !blurb.isEmpty else { return }
        post(path: "/answer", body: "timestamp=\(timestamp)&blurb=\(blurb.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? blurb)")
        editingTimestamp = nil
        editTexts[timestamp] = nil
        Task { await refresh() }
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

// TagTextField wraps a TextField with tag autocomplete suggestions.
// When the user types a `#` followed by characters, matching tags appear
// as tappable chips below the field.
struct TagTextField: View {
    let placeholder: String
    @Binding var text: String
    let allTags: [String]

    private var currentTagPrefix: String? {
        // Find the word being typed after the last # or space.
        guard let hashRange = text.range(of: "#", options: .backwards) else { return nil }
        let afterHash = String(text[hashRange.upperBound...])
        // If there's a space after the #, the user finished typing this tag.
        if afterHash.contains(" ") { return nil }
        return afterHash.lowercased()
    }

    private var suggestions: [String] {
        guard let prefix = currentTagPrefix else { return [] }
        if prefix.isEmpty {
            // Just typed #, show all tags.
            return Array(allTags.prefix(8))
        }
        return allTags
            .filter { $0.lowercased().hasPrefix(prefix) && $0.lowercased() != prefix }
            .prefix(8)
            .map { $0 }
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            TextField(placeholder, text: $text)
                .textFieldStyle(.roundedBorder)
                .autocorrectionDisabled()
                .textInputAutocapitalization(.never)
                .toolbar {
                    ToolbarItemGroup(placement: .keyboard) {
                        Button("#") {
                            text.append("#")
                        }
                        .font(.body.bold())
                        Spacer()
                    }
                }

            if !suggestions.isEmpty {
                ScrollView(.horizontal, showsIndicators: false) {
                    HStack(spacing: 6) {
                        ForEach(suggestions, id: \.self) { tag in
                            Button {
                                insertTag(tag)
                            } label: {
                                Text("#\(tag)")
                                    .font(.caption)
                                    .padding(.horizontal, 8)
                                    .padding(.vertical, 4)
                                    .background(Color.blue.opacity(0.15))
                                    .foregroundStyle(.blue)
                                    .clipShape(Capsule())
                            }
                            .buttonStyle(.plain)
                        }
                    }
                }
            }
        }
    }

    private func insertTag(_ tag: String) {
        // Replace the current partial tag with the full tag.
        guard let hashRange = text.range(of: "#", options: .backwards) else { return }
        text = String(text[..<hashRange.lowerBound]) + "#\(tag) "
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
