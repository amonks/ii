import SwiftUI

struct SearchTab: View {
    @EnvironmentObject var nodeManager: NodeManager
    @State private var query = ""
    @State private var allPings: [Ping] = []

    private var results: [Ping] {
        if query.isEmpty { return allPings }
        return allPings.filter { $0.blurb.localizedCaseInsensitiveContains(query) }
    }

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
            .searchable(text: $query, prompt: "Filter by tag or text")
            .task { await refresh() }
            .refreshable { await refresh() }
        }
    }

    private func refresh() async {
        guard nodeManager.isRunning,
              let url = URL(string: "\(nodeManager.baseURL)/sync/pull?since=0"),
              let (data, _) = try? await URLSession.shared.data(from: url)
        else { return }

        struct SyncPayload: Codable { let pings: [Ping]? }
        guard let payload = try? JSONDecoder().decode(SyncPayload.self, from: data) else { return }

        await MainActor.run {
            allPings = (payload.pings ?? [])
                .filter { !$0.blurb.isEmpty }
                .sorted { $0.timestamp > $1.timestamp }
        }
    }
}
