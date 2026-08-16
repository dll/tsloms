// Collect unique chunks from routes map + resolve hashes from index raw html
const fs = require('fs');
let mapText = fs.readFileSync('_routes_map.txt','utf8');
let html = fs.readFileSync('_index_raw.html','utf8');
const chunks = new Set();
for (const line of mapText.split('\n')) {
  const m = line.match(/chunk-[a-f0-9]+/g);
  if (m) m.forEach(x => chunks.add(x));
}
// Also search app.js for chunk refs
let app = fs.readFileSync('_app.js','utf8');
const re = /chunk-[a-f0-9]+/g; let mm;
while ((mm = re.exec(app))) chunks.add(mm[0]);
let out = [];
for (const c of chunks) {
  const id = c.replace('chunk-','');
  const esc = c.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const hm = html.match(new RegExp('\\"' + esc + '\\":\\"([a-f0-9]+)\"'));
  if (hm) out.push(`${c}.${hm[1]}.js`);
}
fs.writeFileSync('_all_chunks.txt', out.join('\n'));
console.log('total chunks:', out.length);
