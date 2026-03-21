import CoreLocation
import MapLibre
import SwiftUI

/// A SwiftUI wrapper around MLNMapView that displays track data from the
/// local node as MVT vector tiles.
struct TrackMapView: UIViewRepresentable {
    let nodePort: Int
    let clientID: String
    let detail: Double

    func makeCoordinator() -> Coordinator {
        Coordinator()
    }

    func makeUIView(context: Context) -> MLNMapView {
        let styleURL = URL(string: "https://tiles.openfreemap.org/styles/liberty")!
        let mapView = MLNMapView(frame: .zero, styleURL: styleURL)
        mapView.autoresizingMask = [.flexibleWidth, .flexibleHeight]
        mapView.showsUserLocation = true
        mapView.setZoomLevel(14, animated: false)
        mapView.delegate = context.coordinator
        context.coordinator.nodePort = nodePort
        context.coordinator.clientID = clientID
        context.coordinator.detail = detail
        context.coordinator.didSetInitialLocation = false
        return mapView
    }

    func updateUIView(_ mapView: MLNMapView, context: Context) {
        if context.coordinator.detail != detail {
            context.coordinator.detail = detail
            context.coordinator.reloadTiles(on: mapView)
        }
    }

    @MainActor
    final class Coordinator: NSObject, MLNMapViewDelegate {
        var nodePort: Int = 0
        var clientID: String = ""
        var detail: Double = 10
        var didSetInitialLocation = false
        private var sourceAdded = false

        nonisolated func mapView(_ mapView: MLNMapView, didFinishLoading style: MLNStyle) {
            MainActor.assumeIsolated {
                addTrackLayer(to: style)
            }
        }

        nonisolated func mapView(_ mapView: MLNMapView, didUpdate userLocation: MLNUserLocation?) {
            MainActor.assumeIsolated {
                guard !didSetInitialLocation,
                      let location = userLocation?.coordinate,
                      CLLocationCoordinate2DIsValid(location) else { return }
                didSetInitialLocation = true
                mapView.setCenter(location, zoomLevel: 14, animated: false)
            }
        }

        private func addTrackLayer(to style: MLNStyle) {
            guard !sourceAdded else { return }
            sourceAdded = true

            let detailStr = String(format: "%.1f", detail)
            let tileURL = "http://127.0.0.1:\(nodePort)/tiles/{z}/{x}/{y}?client=\(clientID)&detail=\(detailStr)"
            let source = MLNVectorTileSource(
                identifier: "track-source",
                tileURLTemplates: [tileURL],
                options: [
                    .minimumZoomLevel: NSNumber(value: 0),
                    .maximumZoomLevel: NSNumber(value: 20),
                ]
            )
            style.addSource(source)

            let lineLayer = MLNLineStyleLayer(identifier: "track-line", source: source)
            lineLayer.sourceLayerIdentifier = "track"
            lineLayer.lineColor = NSExpression(forConstantValue: UIColor(red: 0, green: 0.478, blue: 1, alpha: 1))
            lineLayer.lineWidth = NSExpression(forConstantValue: NSNumber(value: 3))
            lineLayer.lineOpacity = NSExpression(forConstantValue: NSNumber(value: 0.85))
            lineLayer.lineCap = NSExpression(forConstantValue: "round")
            lineLayer.lineJoin = NSExpression(forConstantValue: "round")
            style.addLayer(lineLayer)
        }

        func reloadTiles(on mapView: MLNMapView?) {
            guard let style = mapView?.style,
                  let source = style.source(withIdentifier: "track-source") as? MLNVectorTileSource else {
                return
            }
            let layer = style.layer(withIdentifier: "track-line")
            if let layer { style.removeLayer(layer) }
            style.removeSource(source)
            sourceAdded = false
            addTrackLayer(to: style)
        }
    }
}
