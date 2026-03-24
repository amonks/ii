import SwiftUI

struct ContentView: View {
    @EnvironmentObject var nodeManager: NodeManager

    var body: some View {
        TabView {
            PingsTab()
                .tabItem {
                    Label("Pings", systemImage: "bell.badge")
                }

            SearchTab()
                .tabItem {
                    Label("Search", systemImage: "magnifyingglass")
                }

            GraphsTab()
                .tabItem {
                    Label("Graphs", systemImage: "chart.bar")
                }

            SettingsTab()
                .tabItem {
                    Label("Settings", systemImage: "gear")
                }
        }
    }
}
