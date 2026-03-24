import SwiftUI
import WebKit

struct SearchTab: View {
    @EnvironmentObject var nodeManager: NodeManager

    var body: some View {
        NavigationView {
            WebViewWrapper(url: "\(nodeManager.baseURL)/search")
                .navigationTitle("Search")
        }
    }
}

struct WebViewWrapper: UIViewRepresentable {
    let url: String

    func makeUIView(context: Context) -> WKWebView {
        let webView = WKWebView()
        if let url = URL(string: url) {
            webView.load(URLRequest(url: url))
        }
        return webView
    }

    func updateUIView(_ webView: WKWebView, context: Context) {}
}
