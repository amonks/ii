import MapLibre
import SwiftUI

/// The map tab: renders a full-screen MapLibre map with the track vector
/// tile source, and listens for SSE tile-updated events to reload tiles.
struct MapTab: View {
    let nodePort: Int
    let clientID: String
    let detail: Double

    @State private var mapViewRef: MLNMapView?
    @State private var coordinatorRef: TrackMapView.Coordinator?
    @State private var eventClient: EventClient?
    @State private var listenTask: Task<Void, Never>?

    var body: some View {
        TrackMapView(nodePort: nodePort, clientID: clientID, detail: detail)
            .ignoresSafeArea()
            .onAppear {
                startListening()
            }
            .onDisappear {
                stopListening()
            }
    }

    private func startListening() {
        guard eventClient == nil else { return }
        let client = EventClient(nodePort: nodePort, clientID: clientID)
        self.eventClient = client
        client.start()

        listenTask = Task {
            for await _ in client.events {
                coordinatorRef?.reloadTiles(on: mapViewRef)
            }
        }
    }

    private func stopListening() {
        listenTask?.cancel()
        listenTask = nil
        eventClient?.stop()
        eventClient = nil
    }
}
