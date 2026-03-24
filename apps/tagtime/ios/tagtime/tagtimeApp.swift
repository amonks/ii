import Combine
import SwiftUI
import Mobile

@main
struct TagTimeApp: App {
    @StateObject private var nodeManager = NodeManager()

    var body: some Scene {
        WindowGroup {
            ContentView()
                .environmentObject(nodeManager)
                .onAppear {
                    nodeManager.start()
                    NotificationManager.shared.requestPermission()
                    NotificationManager.shared.scheduleUpcoming(baseURL: nodeManager.baseURL)
                }
        }
    }
}

class NodeManager: ObservableObject {
    @Published var port: Int = 0
    @Published var isRunning = false

    var baseURL: String {
        "http://127.0.0.1:\(port)"
    }

    func start() {
        guard !isRunning else { return }

        let documentsDir = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first!
        let dbPath = documentsDir.appendingPathComponent("tagtime.db").path

        let config: [String: Any] = [
            "db_path": dbPath,
            "node_id": UIDevice.current.identifierForVendor?.uuidString ?? "ios",
            "default_seed": 11193462,
            "default_period_secs": 2700
        ]

        guard let configData = try? JSONSerialization.data(withJSONObject: config) else {
            return
        }

        var error: NSError?
        var p: Int = 0
        MobileStart(configData, &p, &error)
        if let error = error {
            print("Failed to start node: \(error)")
            return
        }

        port = p
        isRunning = true
    }

    func stop() {
        MobileStop()
        isRunning = false
    }

    deinit {
        stop()
    }
}
