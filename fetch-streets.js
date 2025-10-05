// Node.js script to fetch streets from Overpass API
import fs from "fs";
import fetch from "node-fetch";
import { featureCollection } from "@turf/helpers";
import { flattenEach } from "@turf/meta";

// Read the Greater Boston boundary GeoJSON
const gj = JSON.parse(fs.readFileSync("greater_boston_polygon.geojson", "utf8"));

// Flatten MultiPolygons to rings
const rings = [];
flattenEach(gj, (feat) => {
  const coords = feat.geometry.coordinates; // Polygon: [ [ [lon,lat], ... ] , holes...]
  const outers = feat.geometry.type === "Polygon" ? [coords[0]] : coords.map(p => p[0]);
  for (const ring of outers) {
    rings.push(ring.map(([lon, lat]) => `${lat} ${lon}`).join(" "));
  }
});

// Overpass supports multiple rings by OR-ing multiple queries if needed.
// Here we'll just use the first ring; for multiple rings, repeat the query and merge.
const polyString = rings[0];

const query = `
[out:json][timeout:120];
(
  way[highway][highway!~"^(footway|path|cycleway|steps|track)$"](poly:"${polyString}");
);
out geom tags;
`;

console.log("Fetching streets from Overpass API...");
console.log("Query:", query);

const resp = await fetch("https://overpass-api.de/api/interpreter", {
  method: "POST",
  headers: { "Content-Type": "text/plain" },
  body: query
});

const data = await resp.json();

// Convert Overpass 'elements'→GeoJSON LineString features
const features = (data.elements || [])
  .filter(el => el.type === "way" && el.geometry)
  .map(el => ({
    type: "Feature",
    properties: { id: el.id, ...el.tags },
    geometry: {
      type: "LineString",
      coordinates: el.geometry.map(g => [g.lon, g.lat])
    }
  }));

const out = { type: "FeatureCollection", features };
fs.writeFileSync("greater_boston_streets.geojson", JSON.stringify(out));
console.log(`Saved ${features.length} street segments to greater_boston_streets.geojson`);
