const fs = require('fs');
let html = fs.readFileSync('_index_raw.html','utf8');
// The manifest maps chunk-id -> hash. Look at a raw slice.
const i = html.indexOf('chunk-c404d6b0');
console.log('context:', JSON.stringify(html.substr(i-3, 35)));
console.log('---');
const c = html.indexOf('"chunk-');  // find manifest style
if (c>=0) console.log('manifest sample:', html.substr(c, 60));
else {
  // maybe format is chunk-X:"hash" without leading quote
  const d = html.indexOf('chunk-c404d6b0');
  console.log('after:', html.substr(d, 40));
}
