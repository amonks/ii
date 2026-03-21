import CoreLocation
import SwiftProtobuf
import Observation

@Observable
@MainActor
final class LocationLogger {
    private var _backgroundSession: CLBackgroundActivitySession?
    private var serviceSession: CLServiceSession?
    private var updatesTask: Task<Void, Never>?

    private let port: Int
    private let session = URLSession.shared
    private var lastStoredDate: Date?
    private var lastStoredLatitude: Double?
    private var lastStoredLongitude: Double?
    private let heartbeatInterval: TimeInterval = 60

    var storeCount: Int = 0
    var lastWatermark: Int64 = 0

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

    init(port: Int) {
        self.port = port
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
        var point = Breadcrumbs_Point()
        point.timestamp = Int64(location.timestamp.timeIntervalSince1970 * 1_000_000_000)
        point.latitude = location.coordinate.latitude
        point.longitude = location.coordinate.longitude
        point.altitude = location.altitude
        point.ellipsoidalAltitude = location.ellipsoidalAltitude
        point.horizontalAccuracy = location.horizontalAccuracy
        point.verticalAccuracy = location.verticalAccuracy
        point.speed = location.speed
        point.speedAccuracy = location.speedAccuracy
        point.course = location.course
        point.courseAccuracy = location.courseAccuracy
        point.floor = Int32(location.floor?.level ?? 0)
        point.isSimulated = location.sourceInformation?.isSimulatedBySoftware ?? false
        point.isFromAccessory = location.sourceInformation?.isProducedByAccessory ?? false

        var track = Breadcrumbs_Track()
        track.points = [point]

        guard let body = try? track.serializedData() else { return }

        var request = URLRequest(url: URL(string: "http://127.0.0.1:\(port)/ingest")!)
        request.httpMethod = "POST"
        request.httpBody = body
        request.setValue("application/protobuf", forHTTPHeaderField: "Content-Type")

        do {
            let (data, _) = try await session.data(for: request)
            let resp = try Breadcrumbs_IngestResponse(serializedBytes: data)
            lastWatermark = resp.watermark
        } catch {
            // Ingest failed — node may be down. Point is lost.
            // TODO: buffer for retry
        }

        lastStoredDate = location.timestamp
        lastStoredLatitude = (location.coordinate.latitude * 1e5).rounded() / 1e5
        lastStoredLongitude = (location.coordinate.longitude * 1e5).rounded() / 1e5
        storeCount += 1
    }
}
