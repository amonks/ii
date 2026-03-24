import Charts
import SwiftUI

struct TagSummary: Codable, Identifiable {
    let name: String
    let total_secs: Double
    let color: String
    let sparkline: [Double]

    var id: String { name }
}

struct TagTreeNode: Codable, Identifiable {
    let name: String
    let full_path: String
    let own_secs: Double
    let total_secs: Double
    let color: String
    let sparkline: [Double]
    let children: [TagTreeNode]?

    var id: String { full_path }
}

struct TagSummaryResponse: Codable {
    let tags: [TagSummary]
    let tree: [TagTreeNode]?
    let total_secs: Double
}

struct TagsTab: View {
    @EnvironmentObject var nodeManager: NodeManager
    @State private var selectedRange = "7d"
    @State private var tree: [TagTreeNode] = []
    @State private var totalSecs: Double = 0

    private let ranges = ["24h", "7d", "30d", "All"]

    var body: some View {
        NavigationView {
            VStack(spacing: 0) {
                Picker("Range", selection: $selectedRange) {
                    ForEach(ranges, id: \.self) { r in
                        Text(r).tag(r)
                    }
                }
                .pickerStyle(.segmented)
                .padding(.horizontal)
                .padding(.vertical, 8)

                if tree.isEmpty {
                    ContentUnavailableView("No Tags", systemImage: "tag")
                        .frame(maxHeight: .infinity)
                } else {
                    List {
                        ForEach(tree) { node in
                            TagTreeRow(node: node)
                        }
                    }
                    .listStyle(.plain)
                }
            }
            .navigationTitle("Tags")
            .task { await refresh() }
            .onChange(of: selectedRange) { Task { await refresh() } }
            .refreshable { await refresh() }
        }
    }

    private func refresh() async {
        let rangeParam = selectedRange.lowercased()
        guard nodeManager.isRunning,
              let url = URL(string: "\(nodeManager.baseURL)/tags/summary?range=\(rangeParam)"),
              let (data, _) = try? await URLSession.shared.data(from: url)
        else { return }

        guard let response = try? JSONDecoder().decode(TagSummaryResponse.self, from: data) else { return }

        await MainActor.run {
            tree = response.tree ?? []
            totalSecs = response.total_secs
        }
    }
}

struct TagTreeRow: View {
    let node: TagTreeNode

    var body: some View {
        if let children = node.children, !children.isEmpty {
            DisclosureGroup {
                ForEach(children) { child in
                    TagTreeRow(node: child)
                }
            } label: {
                NavigationLink(destination: TagDetailView(tagName: node.full_path)) {
                    TagNodeContent(node: node)
                }
            }
        } else {
            NavigationLink(destination: TagDetailView(tagName: node.full_path)) {
                TagNodeContent(node: node)
            }
        }
    }
}

struct TagNodeContent: View {
    let node: TagTreeNode

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                Circle()
                    .fill(Color(hex: node.color) ?? .gray)
                    .frame(width: 10, height: 10)
                Text(node.name)
                    .font(.headline)
                Spacer()
                Text(formatDuration(node.total_secs))
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
            }
            Chart(Array(node.sparkline.enumerated()), id: \.offset) { idx, val in
                BarMark(
                    x: .value("i", idx),
                    y: .value("v", val)
                )
                .foregroundStyle(Color(hex: node.color) ?? .blue)
            }
            .chartXAxis(.hidden)
            .chartYAxis(.hidden)
            .frame(height: 30)
        }
        .padding(.vertical, 4)
    }

    private func formatDuration(_ secs: Double) -> String {
        let h = Int(secs) / 3600
        let m = (Int(secs) % 3600) / 60
        if h > 0 {
            return "\(h)h \(m)m"
        }
        return "\(m)m"
    }
}

extension Color {
    init?(hex: String) {
        var hex = hex
        if hex.hasPrefix("#") {
            hex.removeFirst()
        }
        guard hex.count == 6,
              let int = UInt64(hex, radix: 16)
        else { return nil }
        let r = Double((int >> 16) & 0xFF) / 255
        let g = Double((int >> 8) & 0xFF) / 255
        let b = Double(int & 0xFF) / 255
        self.init(red: r, green: g, blue: b)
    }
}
