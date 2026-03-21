import Foundation

/// Connects to the node's /events SSE endpoint and delivers tile-updated
/// notifications as an AsyncSequence.
final class EventClient: NSObject, URLSessionDataDelegate, @unchecked Sendable {
    private let url: URL
    private var session: URLSession?
    private var dataTask: URLSessionDataTask?
    private var buffer = Data()

    private let continuation: AsyncStream<TileUpdate>.Continuation
    let events: AsyncStream<TileUpdate>

    struct TileUpdate: Sendable {
        let z: Int
        let x: Int
        let y: Int
    }

    init(nodePort: Int, clientID: String) {
        self.url = URL(string: "http://127.0.0.1:\(nodePort)/events?client=\(clientID)")!

        var cont: AsyncStream<TileUpdate>.Continuation!
        self.events = AsyncStream { cont = $0 }
        self.continuation = cont

        super.init()
    }

    func start() {
        let config = URLSessionConfiguration.default
        config.timeoutIntervalForRequest = .infinity
        config.timeoutIntervalForResource = .infinity
        session = URLSession(configuration: config, delegate: self, delegateQueue: nil)

        var request = URLRequest(url: url)
        request.setValue("text/event-stream", forHTTPHeaderField: "Accept")
        request.timeoutInterval = .infinity
        dataTask = session?.dataTask(with: request)
        dataTask?.resume()
    }

    func stop() {
        dataTask?.cancel()
        dataTask = nil
        session?.invalidateAndCancel()
        session = nil
        continuation.finish()
    }

    // MARK: - URLSessionDataDelegate

    nonisolated func urlSession(_ session: URLSession, dataTask: URLSessionDataTask, didReceive data: Data) {
        buffer.append(data)
        processBuffer()
    }

    nonisolated func urlSession(_ session: URLSession, task: URLSessionTask, didCompleteWithError error: (any Error)?) {
        // Connection dropped — the caller can reconnect.
        continuation.finish()
    }

    // MARK: - SSE parsing

    private func processBuffer() {
        guard let text = String(data: buffer, encoding: .utf8) else { return }
        let lines = text.components(separatedBy: "\n")

        // Keep the last incomplete line in the buffer.
        var eventType: String?
        var dataLine: String?
        var consumed = 0

        for line in lines {
            consumed += line.utf8.count + 1 // +1 for \n

            if line.hasPrefix("event: ") {
                eventType = String(line.dropFirst("event: ".count))
            } else if line.hasPrefix("data: ") {
                dataLine = String(line.dropFirst("data: ".count))
            } else if line.isEmpty {
                // End of event block.
                if eventType == "tile-updated", let json = dataLine {
                    if let update = parseTileUpdate(json) {
                        continuation.yield(update)
                    }
                }
                eventType = nil
                dataLine = nil
            }
        }

        // Keep only unconsumed data.
        if consumed <= buffer.count {
            buffer = buffer.suffix(from: buffer.index(buffer.startIndex, offsetBy: consumed))
        } else {
            buffer = Data()
        }
    }

    private func parseTileUpdate(_ json: String) -> TileUpdate? {
        guard let data = json.data(using: .utf8),
              let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let z = obj["z"] as? Int,
              let x = obj["x"] as? Int,
              let y = obj["y"] as? Int else {
            return nil
        }
        return TileUpdate(z: z, x: x, y: y)
    }
}
