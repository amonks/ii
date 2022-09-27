(() => {
  // index.ts
  window.initMap = initMap;
  function initMap() {
    const places = window.places;
    if (!Array.isArray(places) || !places.length) {
      throw Error("no places found in html");
    }
    const div = document.getElementById("map");
    const map = new google.maps.Map(div, {
      zoom: 4,
      center: { lat: Number(places[0].Lat), lng: Number(places[0].Lng) },
      styles: [
        {
          featureType: "landscape",
          elementType: "labels",
          stylers: [{ visibility: "off" }],
        },
      ],
    });
    for (const place of places) {
      new google.maps.Marker({
        position: { lat: Number(place.Lat), lng: Number(place.Lng) },
        map,
      });
    }
  }
})();
