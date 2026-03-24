import Charts
import SwiftUI

struct GraphBucket: Codable {
    let start: String
    let end: String
    let tags: [String: Double]
}

struct GraphResponse: Codable {
    let buckets: [GraphBucket]
    let all_tags: [String]
    let window: String
}

struct TagEntry: Identifiable {
    let id = UUID()
    let date: Date
    let tag: String
    let percent: Double
}

struct GraphsTab: View {
    @EnvironmentObject var nodeManager: NodeManager
    @State private var window = "day"
    @State private var graphData: GraphResponse?
    @State private var entries: [TagEntry] = []

    private let windows = ["hour", "day", "week"]
    private static let iso8601: ISO8601DateFormatter = {
        let f = ISO8601DateFormatter()
        f.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return f
    }()
    private static let iso8601NoFrac: ISO8601DateFormatter = {
        let f = ISO8601DateFormatter()
        f.formatOptions = [.withInternetDateTime]
        return f
    }()

    var body: some View {
        NavigationView {
            VStack {
                Picker("Window", selection: $window) {
                    ForEach(windows, id: \.self) { w in
                        Text(w.capitalized).tag(w)
                    }
                }
                .pickerStyle(.segmented)
                .padding(.horizontal)

                if entries.isEmpty {
                    ContentUnavailableView("No Data", systemImage: "chart.bar")
                        .frame(maxHeight: .infinity)
                } else {
                    Chart(entries) { entry in
                        BarMark(
                            x: .value("Date", entry.date, unit: chartUnit),
                            y: .value("Percent", entry.percent)
                        )
                        .foregroundStyle(by: .value("Tag", entry.tag))
                    }
                    .chartYAxisLabel("%")
                    .padding()
                }
            }
            .navigationTitle("Graphs")
            .task { await refresh() }
            .onChange(of: window) { Task { await refresh() } }
            .refreshable { await refresh() }
        }
    }

    private var chartUnit: Calendar.Component {
        switch window {
        case "hour": return .hour
        case "week": return .weekOfYear
        default: return .day
        }
    }

    private func refresh() async {
        guard nodeManager.isRunning,
              let url = URL(string: "\(nodeManager.baseURL)/graphs/data?window=\(window)"),
              let (data, _) = try? await URLSession.shared.data(from: url)
        else { return }

        guard let response = try? JSONDecoder().decode(GraphResponse.self, from: data) else { return }

        var built: [TagEntry] = []
        for bucket in response.buckets {
            guard let date = Self.iso8601.date(from: bucket.start)
                    ?? Self.iso8601NoFrac.date(from: bucket.start)
            else { continue }
            for (tag, pct) in bucket.tags {
                if pct > 0 {
                    built.append(TagEntry(date: date, tag: tag, percent: pct))
                }
            }
        }

        await MainActor.run {
            graphData = response
            entries = built
        }
    }
}
