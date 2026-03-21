//
//  maplogTests.swift
//  maplogTests
//
//  Created by Andrew Monks on 3/19/26.
//

import Testing
import SwiftData
import CoreLocation
@testable import maplog

func makeContainer() throws -> ModelContainer {
    let config = ModelConfiguration(isStoredInMemoryOnly: true)
    return try ModelContainer(for: LocationRecord.self, configurations: config)
}

func makeLocation(
    latitude: Double = 37.334900,
    longitude: Double = -122.009020,
    timestamp: Date = .now
) -> CLLocation {
    CLLocation(
        coordinate: CLLocationCoordinate2D(latitude: latitude, longitude: longitude),
        altitude: 10,
        horizontalAccuracy: 5,
        verticalAccuracy: 3,
        course: 90,
        courseAccuracy: 5,
        speed: 1.5,
        speedAccuracy: 0.5,
        timestamp: timestamp
    )
}

struct LocationRecordTests {

    @Test func storeRoundTrip() async throws {
        let container = try makeContainer()
        let context = await container.mainContext

        let record = LocationRecord(
            timestamp: .now,
            latitude: 37.334900,
            longitude: -122.009020,
            altitude: 10,
            ellipsoidalAltitude: 12,
            horizontalAccuracy: 5,
            verticalAccuracy: 3,
            speed: 1.5,
            speedAccuracy: 0.5,
            course: 90,
            courseAccuracy: 5,
            floor: nil,
            isSimulatedBySoftware: false,
            isProducedByAccessory: false
        )

        await MainActor.run { context.insert(record) }
        try await MainActor.run { try context.save() }

        let descriptor = FetchDescriptor<LocationRecord>()
        let results = try await MainActor.run { try context.fetch(descriptor) }
        #expect(results.count == 1)
        #expect(results[0].latitude == 37.334900)
        #expect(results[0].longitude == -122.009020)
        #expect(results[0].floor == nil)
    }

    @Test func allFieldsPersist() async throws {
        let container = try makeContainer()
        let context = await container.mainContext

        let record = LocationRecord(
            timestamp: Date(timeIntervalSince1970: 1000),
            latitude: 51.5,
            longitude: -0.1,
            altitude: 100,
            ellipsoidalAltitude: 105,
            horizontalAccuracy: 10,
            verticalAccuracy: 8,
            speed: 3.0,
            speedAccuracy: 1.0,
            course: 180,
            courseAccuracy: 10,
            floor: 3,
            isSimulatedBySoftware: true,
            isProducedByAccessory: true
        )

        await MainActor.run { context.insert(record) }
        try await MainActor.run { try context.save() }

        let descriptor = FetchDescriptor<LocationRecord>()
        let fetched = try await MainActor.run { try context.fetch(descriptor) }.first!
        #expect(fetched.altitude == 100)
        #expect(fetched.ellipsoidalAltitude == 105)
        #expect(fetched.speedAccuracy == 1.0)
        #expect(fetched.courseAccuracy == 10)
        #expect(fetched.floor == 3)
        #expect(fetched.isSimulatedBySoftware == true)
        #expect(fetched.isProducedByAccessory == true)
    }
}

@Suite(.serialized)
struct LocationLoggerTests {

    @Test func storeWritesToDatabase() async throws {
        let container = try makeContainer()
        let logger = await LocationLogger(container: container)
        let location = makeLocation()

        await logger.store(location)

        let count = try await MainActor.run {
            try container.mainContext.fetchCount(FetchDescriptor<LocationRecord>())
        }
        #expect(count == 1)
        let sc = await logger.storeCount
        #expect(sc == 1)
    }

    @Test func shouldStoreAllowsFirstUpdate() async throws {
        let container = try makeContainer()
        let logger = await LocationLogger(container: container)
        let location = makeLocation()

        let result = await logger.shouldStore(location)
        #expect(result == true)
    }

    @Test func shouldStoreSkipsDuplicateWithinHeartbeat() async throws {
        let container = try makeContainer()
        let logger = await LocationLogger(container: container)
        let now = Date.now
        let loc1 = makeLocation(timestamp: now)
        let loc2 = makeLocation(timestamp: now.addingTimeInterval(5))

        await logger.store(loc1)
        let result = await logger.shouldStore(loc2)
        #expect(result == false)
    }

    @Test func shouldStoreAllowsDuplicateAfterHeartbeat() async throws {
        let container = try makeContainer()
        let logger = await LocationLogger(container: container)
        let now = Date.now
        let loc1 = makeLocation(timestamp: now)
        let loc2 = makeLocation(timestamp: now.addingTimeInterval(61))

        await logger.store(loc1)
        let result = await logger.shouldStore(loc2)
        #expect(result == true)
    }

    @Test func shouldStoreAllowsMovementWithinHeartbeat() async throws {
        let container = try makeContainer()
        let logger = await LocationLogger(container: container)
        let now = Date.now
        let loc1 = makeLocation(latitude: 37.33490, longitude: -122.00902, timestamp: now)
        // Move ~11m (enough to change at 1e5 rounding)
        let loc2 = makeLocation(latitude: 37.33500, longitude: -122.00902, timestamp: now.addingTimeInterval(1))

        await logger.store(loc1)
        let result = await logger.shouldStore(loc2)
        #expect(result == true)
    }

    @Test func shouldStoreSkipsSubMeterJitter() async throws {
        let container = try makeContainer()
        let logger = await LocationLogger(container: container)
        let now = Date.now
        let loc1 = makeLocation(latitude: 37.334900, longitude: -122.009020, timestamp: now)
        // Sub-meter jitter: differs at 6th decimal place only
        let loc2 = makeLocation(latitude: 37.334901, longitude: -122.009020, timestamp: now.addingTimeInterval(1))

        await logger.store(loc1)
        let result = await logger.shouldStore(loc2)
        #expect(result == false)
    }

    @Test func recordCountIncrements() async throws {
        let container = try makeContainer()
        let logger = await LocationLogger(container: container)
        let now = Date.now

        await logger.store(makeLocation(latitude: 37.334, timestamp: now))
        await logger.store(makeLocation(latitude: 37.335, timestamp: now.addingTimeInterval(1)))

        let count = await logger.storeCount
        #expect(count == 2)
    }
}

