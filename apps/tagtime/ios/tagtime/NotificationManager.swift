import UserNotifications

struct NotificationSettings {
    var soundEnabled: Bool
    var timeSensitive: Bool

    static func load() -> NotificationSettings {
        let defaults = UserDefaults.standard
        return NotificationSettings(
            soundEnabled: defaults.object(forKey: "notif_sound") as? Bool ?? true,
            timeSensitive: defaults.bool(forKey: "notif_timeSensitive")
        )
    }
}

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
        guard let url = URL(string: "\(baseURL)/next-ping?n=64") else { return }

        URLSession.shared.dataTask(with: url) { data, _, error in
            if let error = error {
                print("NextPings fetch error: \(error)")
                return
            }
            guard let data = data else { return }

            struct Response: Codable {
                let timestamps: [Int64]
            }
            guard let response = try? JSONDecoder().decode(Response.self, from: data) else { return }

            self.scheduleNotifications(for: response.timestamps)
        }.resume()
    }

    private func scheduleNotifications(for timestamps: [Int64]) {
        let center = UNUserNotificationCenter.current()
        let settings = NotificationSettings.load()

        center.removePendingNotificationRequests(withIdentifiers:
            timestamps.map { "tagtime-\($0)" }
        )

        for ts in timestamps {
            let date = Date(timeIntervalSince1970: TimeInterval(ts))

            guard date > Date() else { continue }

            let content = UNMutableNotificationContent()
            content.title = "TagTime"
            content.body = "What are you doing right now?"
            content.categoryIdentifier = "TAGTIME_PING"

            if settings.soundEnabled {
                content.sound = .default
            }

            if settings.timeSensitive {
                content.interruptionLevel = .timeSensitive
            }

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
