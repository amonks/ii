//
//  maplogApp.swift
//  maplog
//
//  Created by Andrew Monks on 3/19/26.
//

import SwiftUI
import SwiftData
import Mobile

@main
struct maplogApp: App {
    let container: ModelContainer
    @State private var logger: LocationLogger
    @State private var nodePort: Int = 0
    @Environment(\.scenePhase) private var scenePhase

    /// Random 16-byte hex client ID, regenerated each launch.
    let clientID: String = {
        var bytes = [UInt8](repeating: 0, count: 16)
        _ = SecRandomCopyBytes(kSecRandomDefault, bytes.count, &bytes)
        return bytes.map { String(format: "%02x", $0) }.joined()
    }()

    init() {
        let container = try! ModelContainer(for: LocationRecord.self)
        self.container = container

        let docsDir = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first!
        let dbPath = docsDir.appendingPathComponent("breadcrumbs.db").path

        let config = """
        {"db_path": "\(dbPath)", "capacity": 100000, "upstream": "https://monks.co/breadcrumbs"}
        """

        var port: Int = 0
        var p: Int = 0
        var err: NSError?
        let ok = MobileStart(config.data(using: .utf8), &p, &err)
        if !ok {
            fatalError("failed to start node: \(err?.localizedDescription ?? "unknown error")")
        }
        port = p

        self._nodePort = State(wrappedValue: port)
        self._logger = State(wrappedValue: LocationLogger(port: port))
    }

    var body: some Scene {
        WindowGroup {
            ContentView(logger: logger, nodePort: nodePort, clientID: clientID)
                .onAppear {
                    if logger.isEnabled { logger.start() }
                }
        }
        .modelContainer(container)
        .onChange(of: scenePhase) { _, newPhase in
            if newPhase == .active {
                // Flush unsent points to upstream on foreground.
                Task {
                    var request = URLRequest(url: URL(string: "http://127.0.0.1:\(nodePort)/flush")!)
                    request.httpMethod = "POST"
                    _ = try? await URLSession.shared.data(for: request)
                }
            }
        }
        // Note: we intentionally do NOT stop the Go node on background.
        // The app tracks location in the background via CLBackgroundActivitySession,
        // so the node must remain running to accept ingest POSTs.
    }
}
