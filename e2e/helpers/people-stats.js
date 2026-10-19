// A made-up answer of /api/library/people-stats: ten people over eight
// years, some photographed together, and their faces in clusters, so the
// People topic of the statistics can be drawn without face recognition.

const NAMES = ['Anna', 'Ben', 'Cleo', 'Dan', 'Eva', 'Finn', 'Gina', 'Hugo', 'Ida', 'Jon'];

function rand(seed) {
  let s = seed;
  return () => {
    s = (s * 1664525 + 1013904223) % 4294967296;
    return s / 4294967296;
  };
}

export function fakePeopleStats(libraryID) {
  const r = rand(7);
  const people = NAMES.map((name, i) => ({ name, photos: 0, lib: 0, cover: i + 1 }));
  const appearances = { person: [], lib: [], photo: [], date: [] };
  const faceSpace = { face: [], lib: [], photo: [], date: [], person: [], x: [], y: [], z: [], faces: 0, axes: 'people' };
  const centres = NAMES.map(() => [r() * 1.6 - 0.8, r() * 1.6 - 0.8, r() * 1.6 - 0.8]);
  let photo = 0;
  for (let year = 2018; year <= 2025; year++) {
    for (let k = 0; k < 40; k++) {
      const id = `photo${photo++}`;
      const date = `${year}-${String(1 + Math.floor(r() * 12)).padStart(2, '0')}-15T12:00:00`;
      const inIt = new Set([Math.floor(Math.pow(r(), 2) * NAMES.length)]);
      if (r() < 0.4) inIt.add(Math.floor(Math.pow(r(), 2) * NAMES.length));
      for (const p of inIt) {
        appearances.person.push(p); appearances.lib.push(0); appearances.photo.push(id); appearances.date.push(date);
        people[p].photos++;
        const c = centres[p];
        faceSpace.face.push(faceSpace.face.length + 1); faceSpace.lib.push(0); faceSpace.photo.push(id); faceSpace.date.push(date);
        faceSpace.person.push(p);
        faceSpace.x.push(c[0] + (r() - 0.5) * 0.25); faceSpace.y.push(c[1] + (r() - 0.5) * 0.25); faceSpace.z.push(c[2] + (r() - 0.5) * 0.25);
      }
    }
  }
  for (let k = 0; k < 120; k++) {
    faceSpace.face.push(faceSpace.face.length + 1); faceSpace.lib.push(0); faceSpace.photo.push(`photo${k}`); faceSpace.date.push('');
    faceSpace.person.push(-1);
    faceSpace.x.push(r() * 2 - 1); faceSpace.y.push(r() * 2 - 1); faceSpace.z.push(r() * 2 - 1);
  }
  faceSpace.faces = faceSpace.face.length;
  const order = people.map((p, i) => i).sort((a, b) => people[b].photos - people[a].photos);
  const rank = new Map(order.map((p, i) => [p, i]));
  appearances.person = appearances.person.map(p => rank.get(p));
  faceSpace.person = faceSpace.person.map(p => (p < 0 ? p : rank.get(p)));
  const periods = ['2018', '2019', '2020', '2021', '2022', '2023', '2024', '2025'];
  return {
    granularity: 'year',
    libraries: [libraryID],
    people: order.map(i => people[i]),
    appearances,
    crowd: {
      buckets: [
        { label: 'No face', min: 0, max: 0 }, { label: '1 face', min: 1, max: 1 }, { label: '2 faces', min: 2, max: 2 },
        { label: '3 to 5 faces', min: 3, max: 5 }, { label: '6 or more', min: 6, max: -1 }],
      total: [400, 210, 80, 30, 6],
      periods,
      counts: periods.map((_, i) => [50, 26 + i, 10, 4, i % 2]),
      photos: 726,
    },
    faceSpace,
  };
}
