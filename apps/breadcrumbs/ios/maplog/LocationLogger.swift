import CoreLocation
import SwiftData
import Observation

@Observable
@MainActor
final class LocationLogger {
    private var _backgroundSession: CLBackgroundActivitySession?
    private var serviceSession: CLServiceSession?
    private var updatesTask: Task<Void, Never>?

    private let container: ModelContainer
    private var lastStoredDate: Date?
    private var lastStoredLatitude: Double?
    private var lastStoredLongitude: Double?
    private let heartbeatInterval: TimeInterval = 60

    var storeCount: Int = 0

    private static let isEnabledKey = "loggingEnabled"

    var isEnabled: Bool {
        didSet {
            UserDefaults.standard.set(isEnabled, forKey: Self.isEnabledKey)
            if isEnabled {
                start()
            } else {
                stop()
            }
        }
    }

    init(container: ModelContainer) {
        self.container = container
        if UserDefaults.standard.object(forKey: Self.isEnabledKey) == nil {
            self.isEnabled = true
        } else {
            self.isEnabled = UserDefaults.standard.bool(forKey: Self.isEnabledKey)
        }
    }

    func start() {
        guard updatesTask == nil else { return }

        serviceSession = CLServiceSession(authorization: .always)
        _backgroundSession = CLBackgroundActivitySession()

        updatesTask = Task {
            let updates = CLLocationUpdate.liveUpdates(.default)
            do {
                for try await update in updates {
                    guard let location = update.location else { continue }
                    if !self.shouldStore(location) { continue }
                    await self.store(location)
                }
            } catch {
                // Stream ended (e.g. authorization revoked). Nothing to do.
            }
        }
    }

    func stop() {
        updatesTask?.cancel()
        updatesTask = nil
        _backgroundSession?.invalidate()
        _backgroundSession = nil
        serviceSession = nil
    }

    func shouldStore(_ location: CLLocation) -> Bool {
        let lat = (location.coordinate.latitude * 1e5).rounded() / 1e5
        let lon = (location.coordinate.longitude * 1e5).rounded() / 1e5
        let changed = lat != lastStoredLatitude || lon != lastStoredLongitude
        if !changed,
           let last = lastStoredDate,
           location.timestamp.timeIntervalSince(last) < heartbeatInterval {
            return false
        }
        return true
    }

    func store(_ location: CLLocation) async {
        let record = LocationRecord(
            timestamp: location.timestamp,
            latitude: location.coordinate.latitude,
            longitude: location.coordinate.longitude,
            altitude: location.altitude,
            ellipsoidalAltitude: location.ellipsoidalAltitude,
            horizontalAccuracy: location.horizontalAccuracy,
            verticalAccuracy: location.verticalAccuracy,
            speed: location.speed,
            speedAccuracy: location.speedAccuracy,
            course: location.course,
            courseAccuracy: location.courseAccuracy,
            floor: location.floor?.level,
            isSimulatedBySoftware: location.sourceInformation?.isSimulatedBySoftware ?? false,
            isProducedByAccessory: location.sourceInformation?.isProducedByAccessory ?? false
        )

        let context = container.mainContext
        context.insert(record)
        try? context.save()

        lastStoredDate = record.timestamp
        lastStoredLatitude = (record.latitude * 1e5).rounded() / 1e5
        lastStoredLongitude = (record.longitude * 1e5).rounded() / 1e5
        storeCount += 1
    }
}
