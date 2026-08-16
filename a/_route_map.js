// 提取 app.js 中所有 ./xxx 路由组件 到 chunk 的映射
const fs = require('fs');
let c = fs.readFileSync('_app.js', 'utf8');

// 匹配  "./(path)":[moduleId, 9, "chunk-xxx", ...]
const re = /\.\/([^"]+?)":\[("[0-9a-f]+"),9,"(chunk-[a-f0-9]+)"(?:,"(chunk-[a-f0-9]+)")?\]/g;
let m, out = [];
while ((m = re.exec(c))) {
  out.push({ path: m[1], mod: m[2], chunks: [m[3], m[4]].filter(Boolean) });
}
// 去重按 path
const map = new Map();
for (const o of out) if (!map.has(o.path)) map.set(o.path, o);
const lines = [...map.values()].map(o => `${o.path.padEnd(55)} :: ${o.mod} :: ${o.chunks.join(', ')}`).sort();
fs.writeFileSync('_routes_map.txt', lines.join('\n'));
console.log(lines.join('\n'));
