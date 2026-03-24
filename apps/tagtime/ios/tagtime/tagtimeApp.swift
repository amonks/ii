import Combine
import SwiftUI
import UserNotifications
import Mobile

@main
struct TagTimeApp: App {
    @StateObject private var nodeManager = NodeManager()
    @StateObject private var navigation = NavigationState()
    @UIApplicationDelegateAdaptor(AppDelegate.self) var appDelegate

    var body: some Scene {
        WindowGroup {
            ContentView()
                .environmentObject(nodeManager)
                .environmentObject(navigation)
                .onAppear {
                    appDelegate.navigation = navigation
                    nodeManager.start()
                    NotificationManager.shared.requestPermission()
                    NotificationManager.shared.scheduleUpcoming(baseURL: nodeManager.baseURL)
                }
        }
    }
}

class AppDelegate: NSObject, UIApplicationDelegate, UNUserNotificationCenterDelegate {
    var navigation: NavigationState?

    func application(_ application: UIApplication, didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?) -> Bool {
        UNUserNotificationCenter.current().delegate = self
        return true
    }

    func userNotificationCenter(_ center: UNUserNotificationCenter, didReceive response: UNNotificationResponse, withCompletionHandler completionHandler: @escaping () -> Void) {
        if response.notification.request.content.categoryIdentifier == "TAGTIME_PING" {
            DispatchQueue.main.async {
                self.navigation?.selectedTab = .pings
            }
        }
        completionHandler()
    }

    // Show notifications even when app is in the foreground.
    func userNotificationCenter(_ center: UNUserNotificationCenter, willPresent notification: UNNotification, withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void) {
        completionHandler([.banner, .sound])
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
