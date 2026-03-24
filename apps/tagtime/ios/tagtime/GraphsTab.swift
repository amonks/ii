import SwiftUI
import WebKit

struct GraphsTab: View {
    @EnvironmentObject var nodeManager: NodeManager

    var body: some View {
        NavigationView {
            WebViewWrapper(url: "\(nodeManager.baseURL)/graphs")
                .navigationTitle("Graphs")
        }
    }
}
