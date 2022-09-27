import "htmx.org";
import { styles } from "./styles";

type Htmx = typeof import("htmx.org");
const htmx: Htmx = (window as any).htmx;

console.log(htmx);

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

  const div = document.getElementById("map");
  const map = new google.maps.Map(div, {
    zoom: 13,
    center: { lat: Number(places[0].Lat), lng: Number(places[0].Lng) },
    streetViewControl: false,
    mapTypeControl: false,
    zoomControl: false,
    fullscreenControl: false,
    styles,
  });

  const icon = {
    // size: new google.maps.Size(1, 1),
    scaledSize: new google.maps.Size(8, 8),
    url: "./dot.png",
  };

  for (const place of places) {
    const marker = new google.maps.Marker({
      position: { lat: Number(place.Lat), lng: Number(place.Lng) },
      icon,
      map,
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
