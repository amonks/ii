import SwiftUI

struct SearchTab: View {
    @EnvironmentObject var nodeManager: NodeManager
    @State private var query = ""
    @State private var results: [Ping] = []

    var body: some View {
        NavigationView {
            List(results) { ping in
                VStack(alignment: .leading) {
                    Text(ping.date, style: .relative)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                    Text(ping.blurb)
                }
            }
            .navigationTitle("Search")
            .searchable(text: $query, prompt: "Search pings...")
            .onSubmit(of: .search) { Task { await search() } }
            .onChange(of: query) {
                if query.isEmpty {
                    results = []
                }
            }
        }
    }

    private func search() async {
        guard nodeManager.isRunning, !query.isEmpty else {
            await MainActor.run { results = [] }
            return
        }

        guard let encoded = query.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed),
              let url = URL(string: "\(nodeManager.baseURL)/search/data?q=\(encoded)"),
              let (data, _) = try? await URLSession.shared.data(from: url)
        else { return }

        struct SearchResponse: Codable {
            let query: String
            let results: [Ping]?
        }
        guard let response = try? JSONDecoder().decode(SearchResponse.self, from: data) else { return }

        await MainActor.run {
            results = response.results ?? []
        }
    }
}
