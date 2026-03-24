import SwiftUI

struct TagRenameEntry: Codable, Identifiable {
    let old_name: String
    let new_name: String
    let renamed_at: Int64

    var id: Int64 { renamed_at }

    var date: Date {
        Date(timeIntervalSince1970: TimeInterval(renamed_at))
    }
}

struct TagDetailResponse: Codable {
    let name: String
    let renames: [TagRenameEntry]
    let pings: [Ping]
}

struct TagDetailView: View {
    @EnvironmentObject var nodeManager: NodeManager
    let tagName: String

    @State private var renames: [TagRenameEntry] = []
    @State private var pings: [Ping] = []
    @State private var newName = ""
    @State private var isRenaming = false

    var body: some View {
        List {
            Section("Rename") {
                HStack {
                    TextField("New name", text: $newName)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                    Button("Rename") {
                        Task { await rename() }
                    }
                    .disabled(newName.isEmpty || isRenaming)
                }
            }

            if !renames.isEmpty {
                Section("Rename History") {
                    ForEach(renames) { entry in
                        HStack {
                            Text("\(entry.old_name) → \(entry.new_name)")
                            Spacer()
                            Text(entry.date, style: .date)
                                .font(.caption)
                                .foregroundStyle(.secondary)
                        }
                    }
                }
            }

            Section("Pings (\(pings.count))") {
                ForEach(pings) { ping in
                    VStack(alignment: .leading, spacing: 2) {
                        Text(ping.blurb)
                            .font(.body)
                        Text(ping.date, style: .date) + Text(" ") + Text(ping.date, style: .time)
                    }
                    .font(.caption)
                    .foregroundStyle(.secondary)
                }
            }
        }
        .navigationTitle(tagName)
        .task { await refresh() }
    }

    private func refresh() async {
        let encoded = tagName.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? tagName
        guard nodeManager.isRunning,
              let url = URL(string: "\(nodeManager.baseURL)/tags/\(encoded)"),
              let (data, _) = try? await URLSession.shared.data(from: url)
        else { return }

        guard let response = try? JSONDecoder().decode(TagDetailResponse.self, from: data) else { return }

        await MainActor.run {
            renames = response.renames
            pings = response.pings
        }
    }

    private func rename() async {
        guard nodeManager.isRunning,
              let url = URL(string: "\(nodeManager.baseURL)/tags/rename")
        else { return }

        isRenaming = true
        defer { isRenaming = false }

        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/x-www-form-urlencoded", forHTTPHeaderField: "Content-Type")
        let oldEncoded = tagName.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? tagName
        let newEncoded = newName.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? newName
        let body = "old_name=\(oldEncoded)&new_name=\(newEncoded)"
        request.httpBody = body.data(using: .utf8)

        guard let (_, response) = try? await URLSession.shared.data(for: request),
              let http = response as? HTTPURLResponse,
              http.statusCode == 200
        else { return }

        newName = ""
        await refresh()
    }
}
