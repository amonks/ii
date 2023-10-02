import htmx from "htmx.org";

(window as any).initMap = initMap;

type Place = {
  GoogleMapsURL: string;
  Lat: string;
  Lng: string;
  Title: string;
};

function initMap() {
  const places = (window as any).places as Place[];
  if (!Array.isArray(places) || !places.length) {
    throw Error("no places found in html");
  }

  // h-mart is a reasonably good place to center the map
  const center = places.find((p) => p.Title === "H Mart Chicago")!;

  const div = document.getElementById("map")!;
  const map = new google.maps.Map(div, {
    mapId: "da4892bae6f26cb6",
    zoom: 13,
    center: { lat: Number(center.Lat), lng: Number(center.Lng) },
    streetViewControl: false,
    mapTypeControl: false,
    zoomControl: false,
    fullscreenControl: false,
  });

  for (const place of places) {
    const marker = new google.maps.marker.AdvancedMarkerView({
      map,
      position: { lat: Number(place.Lat), lng: Number(place.Lng) },
      content: document.getElementById(`marker-${place.GoogleMapsURL}`),
    });
    marker.addListener("click", () => {
      console.log(place);
      const id = encodeURIComponent(place.GoogleMapsURL);
      const path = `/places?url=${id}`;
      htmx.ajax("get", path, {
        target: "#sidebar",
        source: "#sidebar",
        swap: "replace",
      });
    });
  }
}
