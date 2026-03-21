//
//  maplogApp.swift
//  maplog
//
//  Created by Andrew Monks on 3/19/26.
//

import SwiftUI
import SwiftData

@main
struct maplogApp: App {
    let container: ModelContainer
    @State private var logger: LocationLogger

    init() {
        let container = try! ModelContainer(for: LocationRecord.self)
        self.container = container
        self._logger = State(wrappedValue: LocationLogger(container: container))
    }

    var body: some Scene {
        WindowGroup {
            ContentView(logger: logger)
                .onAppear {
                    if logger.isEnabled { logger.start() }
                }
        }
        .modelContainer(container)
    }
}
