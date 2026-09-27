// MapMarkers — the photos on the Map, grouped by place (ADR-0039).
//
// MapLibre clusters the points itself. Every point carries its rank (0 is
// the oldest photo), and each cluster keeps the highest rank of its points,
// so the newest photo of a group is known without asking for its leaves.
// The markers are HTML: a round thumbnail of that photo and the count.

const MAP_SOURCE = 'photos';
const MAP_CLUSTER_MAX_ZOOM = 18;

class MapMarkers {
    // photoAt(rank): the photo with this rank (0 is the oldest).
    // onOpen(photos): open these photos, newest first.
    constructor(map, { photoAt, onOpen }) {
        this._map = map;
        this._photoAt = photoAt;
        this._onOpen = onOpen;
        this._markers = new Map(); // key → maplibregl.Marker
        map.on('render', () => {
            if (map.getSource(MAP_SOURCE) && map.isSourceLoaded(MAP_SOURCE)) this._update();
        });
    }

    // Shows these photos, and only these. A new map style drops the
    // source, so it is added again whenever it is missing.
    show(points) {
        for (const marker of this._markers.values()) marker.remove();
        this._markers.clear();
        const data = this._geojson(points);
        const source = this._map.getSource(MAP_SOURCE);
        if (source) {
            source.setData(data);
            return;
        }
        this._map.addSource(MAP_SOURCE, {
            type: 'geojson',
            data,
            cluster: true,
            clusterRadius: 56,
            clusterMaxZoom: MAP_CLUSTER_MAX_ZOOM,
            clusterProperties: { newest: ['max', ['get', 'rank']] },
        });
        // Nothing drawn: the layer only makes MapLibre tile the source, so
        // querySourceFeatures has something to return.
        this._map.addLayer({ id: 'photos-anchor', type: 'circle', source: MAP_SOURCE, paint: { 'circle-opacity': 0, 'circle-radius': 1 } });
    }

    _geojson(points) {
        return {
            type: 'FeatureCollection',
            features: points.map(p => ({
                type: 'Feature',
                geometry: { type: 'Point', coordinates: [p.lon, p.lat] },
                properties: { rank: p.rank },
            })),
        };
    }

    // Adds the markers that came into view and removes the ones that left.
    _update() {
        const next = new Map();
        for (const f of this._map.querySourceFeatures(MAP_SOURCE)) {
            const props = f.properties;
            const key = props.cluster ? `c${props.cluster_id}` : `p${props.rank}`;
            if (next.has(key)) continue; // a feature on a tile edge comes twice
            next.set(key, this._markers.get(key) || this._marker(f));
        }
        for (const [key, marker] of this._markers) {
            if (!next.has(key)) marker.remove();
        }
        for (const [key, marker] of next) {
            if (!this._markers.has(key)) marker.addTo(this._map);
        }
        this._markers = next;
    }

    _marker(feature) {
        const props = feature.properties;
        const count = props.cluster ? props.point_count : 1;
        const newest = this._photoAt(props.cluster ? props.newest : props.rank);

        const el = document.createElement('button');
        el.type = 'button';
        el.className = 'map-marker';
        el.setAttribute('aria-label', markerLabel(newest, count));
        el.innerHTML = `<img src="${LibraryAPI.thumbURL(newest.lib, newest.id)}" alt="" decoding="async">` +
            (count > 1 ? `<span class="map-marker-count">${formatCount(count)}</span>` : '');
        el.addEventListener('click', () => {
            if (props.cluster) this._openCluster(props.cluster_id, feature.geometry.coordinates);
            else this._onOpen([newest]);
        });
        return new maplibregl.Marker({ element: el, anchor: 'center' })
            .setLngLat(feature.geometry.coordinates);
    }

    // Zooms in until the group falls apart. A group that never does — its
    // photos share one spot — opens in the viewer instead.
    async _openCluster(clusterId, center) {
        const source = this._map.getSource(MAP_SOURCE);
        const zoom = await source.getClusterExpansionZoom(clusterId);
        if (zoom <= MAP_CLUSTER_MAX_ZOOM) {
            const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;
            this._map.easeTo({ center, zoom, duration: reduce ? 0 : 240 });
            return;
        }
        const leaves = await source.getClusterLeaves(clusterId, Infinity, 0);
        const photos = leaves.map(f => this._photoAt(f.properties.rank));
        photos.sort((a, b) => b.rank - a.rank);
        this._onOpen(photos);
    }
}

// "12 photos, the newest from 3 May 2024" · "DSCF0042.JPG, 3 May 2024"
function markerLabel(newest, count) {
    const date = newest.taken ? formatTakenDate(newest.taken) : '';
    if (count === 1) return date ? `${newest.name}, ${date}` : newest.name;
    return date ? `${formatCount(count)} photos, the newest from ${date}` : `${formatCount(count)} photos`;
}

function formatTakenDate(iso) {
    const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(iso);
    if (!m) return '';
    return new Date(Date.UTC(+m[1], +m[2] - 1, +m[3]))
        .toLocaleDateString('en-GB', { day: 'numeric', month: 'long', year: 'numeric', timeZone: 'UTC' });
}
