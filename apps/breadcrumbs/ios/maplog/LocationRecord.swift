import Foundation
import SwiftData

@Model
final class LocationRecord {
    var timestamp: Date
    var latitude: Double
    var longitude: Double
    var altitude: Double
    var ellipsoidalAltitude: Double
    var horizontalAccuracy: Double
    var verticalAccuracy: Double
    var speed: Double
    var speedAccuracy: Double
    var course: Double
    var courseAccuracy: Double
    var floor: Int?
    var isSimulatedBySoftware: Bool
    var isProducedByAccessory: Bool

    init(
        timestamp: Date,
        latitude: Double,
        longitude: Double,
        altitude: Double,
        ellipsoidalAltitude: Double,
        horizontalAccuracy: Double,
        verticalAccuracy: Double,
        speed: Double,
        speedAccuracy: Double,
        course: Double,
        courseAccuracy: Double,
        floor: Int?,
        isSimulatedBySoftware: Bool,
        isProducedByAccessory: Bool
    ) {
        self.timestamp = timestamp
        self.latitude = latitude
        self.longitude = longitude
        self.altitude = altitude
        self.ellipsoidalAltitude = ellipsoidalAltitude
        self.horizontalAccuracy = horizontalAccuracy
        self.verticalAccuracy = verticalAccuracy
        self.speed = speed
        self.speedAccuracy = speedAccuracy
        self.course = course
        self.courseAccuracy = courseAccuracy
        self.floor = floor
        self.isSimulatedBySoftware = isSimulatedBySoftware
        self.isProducedByAccessory = isProducedByAccessory
    }
}
