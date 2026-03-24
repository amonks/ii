import Combine
import SwiftUI

enum Tab {
    case pings, search, graphs, settings
}

struct ContentView: View {
    @EnvironmentObject var nodeManager: NodeManager
    @EnvironmentObject var navigation: NavigationState

    var body: some View {
        TabView(selection: $navigation.selectedTab) {
            PingsTab()
                .tabItem {
                    Label("Pings", systemImage: "bell.badge")
                }
                .tag(Tab.pings)

            SearchTab()
                .tabItem {
                    Label("Search", systemImage: "magnifyingglass")
                }
                .tag(Tab.search)

            GraphsTab()
                .tabItem {
                    Label("Graphs", systemImage: "chart.bar")
                }
                .tag(Tab.graphs)

            SettingsTab()
                .tabItem {
                    Label("Settings", systemImage: "gear")
                }
                .tag(Tab.settings)
        }
    }
}

class NavigationState: ObservableObject {
    @Published var selectedTab: Tab = .pings
}
