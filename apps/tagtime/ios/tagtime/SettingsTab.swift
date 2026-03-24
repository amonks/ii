import SwiftUI
import WebKit

struct SettingsTab: View {
    @EnvironmentObject var nodeManager: NodeManager

    var body: some View {
        NavigationView {
            WebViewWrapper(url: "\(nodeManager.baseURL)/settings")
                .navigationTitle("Settings")
        }
    }
}
