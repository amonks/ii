import UserNotifications
import Mobile

class NotificationManager {
    static let shared = NotificationManager()

    private init() {}

    func requestPermission() {
        UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .sound, .badge]) { granted, error in
            if let error = error {
                print("Notification permission error: \(error)")
            }
        }
    }

    func scheduleUpcoming(baseURL: String) {
        // Get next 64 ping times from the Go schedule engine.
        let config: [String: Any] = [
            "default_seed": 11193462,
            "default_period_secs": 2700
        ]
        guard let configData = try? JSONSerialization.data(withJSONObject: config) else { return }

        var error: NSError?
        guard let timestampsData = MobileNextPings(configData, 64, &error) else {
            if let error = error {
                print("NextPings error: \(error)")
            }
            return
        }

        guard let timestamps = try? JSONDecoder().decode([Int64].self, from: timestampsData) else { return }

        let center = UNUserNotificationCenter.current()

        // Remove old tagtime notifications.
        center.removePendingNotificationRequests(withIdentifiers:
            timestamps.map { "tagtime-\($0)" }
        )

        for ts in timestamps {
            let date = Date(timeIntervalSince1970: TimeInterval(ts))

            // Don't schedule for the past.
            guard date > Date() else { continue }

            let content = UNMutableNotificationContent()
            content.title = "TagTime"
            content.body = "What are you doing right now?"
            content.sound = .default
            content.categoryIdentifier = "TAGTIME_PING"

            let trigger = UNTimeIntervalNotificationTrigger(
                timeInterval: date.timeIntervalSinceNow,
                repeats: false
            )

            let request = UNNotificationRequest(
                identifier: "tagtime-\(ts)",
                content: content,
                trigger: trigger
            )

            center.add(request) { error in
                if let error = error {
                    print("Failed to schedule notification: \(error)")
                }
            }
        }
    }
}
