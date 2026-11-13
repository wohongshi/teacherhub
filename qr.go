package main

// qr.go —— 迷你 QR 编码器（纯前端 JS，零依赖）。
// 固定 Version 4（33×33）/ EC level L / mask 0：容量 78 字节，产物链接足够。
// 矩阵布置与格式信息遵循 QR 规范（布局参照 qrcode-generator 的成熟实现）。
// 查看页「📱 扫码」用它画二维码，学生扫码直达产物。

import "net/http"

const qrJS = `function qrRender(canvas, text) {
  var EXP = [], LOG = [];
  (function () {
    var x = 1;
    for (var i = 0; i < 255; i++) { EXP[i] = x; LOG[x] = i; x <<= 1; if (x & 256) x ^= 285; }
    for (var i = 255; i < 512; i++) EXP[i] = EXP[i - 255];
  })();
  function gmul(a, b) { return (a && b) ? EXP[(LOG[a] + LOG[b]) % 255] : 0; }
  function rsGen(n) {
    var g = [1];
    for (var i = 0; i < n; i++) {
      var ng = new Array(g.length + 1).fill(0);
      for (var j = 0; j < g.length; j++) { ng[j] ^= gmul(g[j], 1); ng[j + 1] ^= gmul(g[j], EXP[i]); }
      g = ng;
    }
    return g;
  }
  function rsEncode(data, n) {
    var g = rsGen(n), res = data.concat(new Array(n).fill(0));
    for (var i = 0; i < data.length; i++) {
      var f = res[i]; if (!f) continue;
      for (var j = 0; j < g.length; j++) res[i + j] ^= gmul(g[j], f);
    }
    return res.slice(data.length);
  }
  /* UTF-8 字节 */
  var bytes = [];
  for (var i = 0; i < text.length; i++) {
    var c = text.charCodeAt(i);
    if (c < 0x80) bytes.push(c);
    else if (c < 0x800) { bytes.push(0xc0 | c >> 6, 0x80 | c & 63); }
    else { bytes.push(0xe0 | c >> 12, 0x80 | (c >> 6 & 63), 0x80 | c & 63); }
  }
  if (bytes.length > 78) bytes = bytes.slice(0, 78); /* Version 4-L 容量 */
  /* 位流：模式 0100 + 长度 8bit + 数据 + 终止 + 填充 */
  var bits = [];
  function put(v, n) { for (var i = n - 1; i >= 0; i--) bits.push(v >> i & 1); }
  put(4, 4); put(bytes.length, 8);
  for (var i = 0; i < bytes.length; i++) put(bytes[i], 8);
  var cap = 80 * 8;
  for (var i = 0; i < 4 && bits.length < cap; i++) bits.push(0);
  while (bits.length % 8) bits.push(0);
  var cw = [];
  for (var i = 0; i < bits.length; i += 8) {
    var b = 0; for (var j = 0; j < 8; j++) b = b << 1 | bits[i + j];
    cw.push(b);
  }
  var pads = [0xEC, 0x11], pi = 0;
  while (cw.length < 80) cw.push(pads[pi++ % 2]);
  var all = cw.concat(rsEncode(cw, 20)); /* 80 data + 20 ec */
  /* 矩阵 33×33 */
  var N = 33, M = [];
  for (var r = 0; r < N; r++) M.push(new Array(N).fill(null));
  function finder(r0, c0) {
    for (var r = -1; r <= 7; r++) for (var c = -1; c <= 7; c++) {
      var rr = r0 + r, cc = c0 + c;
      if (rr < 0 || cc < 0 || rr >= N || cc >= N) continue;
      var v = (r >= 0 && r <= 6 && (c === 0 || c === 6)) || (c >= 0 && c <= 6 && (r === 0 || r === 6)) || (r >= 2 && r <= 4 && c >= 2 && c <= 4);
      M[rr][cc] = v ? 1 : 0;
    }
  }
  finder(0, 0); finder(0, 26); finder(26, 0);
  for (var i = 8; i < 25; i++) { M[6][i] = i % 2 ? 0 : 1; M[i][6] = i % 2 ? 0 : 1; }
  for (var r = -2; r <= 2; r++) for (var c = -2; c <= 2; c++) {
    M[26 + r][26 + c] = (Math.abs(r) === 2 || Math.abs(c) === 2 || (r === 0 && c === 0)) ? 1 : 0;
  }
  M[25][8] = 1; /* dark module */
  /* 格式信息（EC=L(01) + mask=000），BCH + XOR 0x5412 */
  function fmtBits(ec, mask) {
    var d = (ec << 3 | mask), v = d << 10;
    for (var i = 4; i >= 0; i--) { if ((v >> (i + 10)) & 1) v ^= 0x537 << i; }
    return ((d << 10) | (v & 0x3ff)) ^ 0x5412;
  }
  var fmt = fmtBits(1, 0);
  for (var i = 0; i < 15; i++) {
    var mod = (fmt >> i) & 1;
    if (i < 6) M[i][8] = mod;
    else if (i < 8) M[i + 1][8] = mod;
    else M[N - 15 + i][8] = mod;
  }
  for (var i = 0; i < 15; i++) {
    var mod = (fmt >> i) & 1;
    if (i < 8) M[8][N - i - 1] = mod;
    else if (i < 9) M[8][15 - i - 1 + 1] = mod;
    else M[8][15 - i - 1] = mod;
  }
  /* 数据蛇形布置 + mask 0 */
  var inc = -1, row = N - 1, bitIndex = 7, byteIndex = 0;
  for (var col = N - 1; col > 0; col -= 2) {
    if (col == 6) col--;
    while (true) {
      for (var c = 0; c < 2; c++) {
        if (M[row][col - c] === null) {
          var dark = false;
          if (byteIndex < all.length) dark = ((all[byteIndex] >>> bitIndex) & 1) === 1;
          if ((row + col - c) % 2 === 0) dark = !dark; /* mask 0 */
          M[row][col - c] = dark ? 1 : 0;
          bitIndex--;
          if (bitIndex === -1) { byteIndex++; bitIndex = 7; }
        }
      }
      row += inc;
      if (row < 0 || row >= N) { row -= inc; inc = -inc; break; }
    }
  }
  /* 画到 canvas（含 4 模块静区） */
  var q = 4, scale = 6, size = (N + q * 2) * scale;
  canvas.width = size; canvas.height = size;
  var g = canvas.getContext('2d');
  g.fillStyle = '#fff'; g.fillRect(0, 0, size, size);
  g.fillStyle = '#000';
  for (var r = 0; r < N; r++) for (var c = 0; c < N; c++) {
    if (M[r][c]) g.fillRect((c + q) * scale, (r + q) * scale, scale, scale);
  }
}
`
const qrHTML = `<div class="lightbox" id="qrov" style="display:none;position:fixed;inset:0;background:rgba(10,14,18,.78);align-items:center;justify-content:center;z-index:99;cursor:zoom-out">
  <div style="background:#fff;border-radius:16px;padding:20px 24px;text-align:center">
    <canvas id="qrcv"></canvas>
    <div style="font-size:12.5px;color:#556;margin-top:10px">手机扫码直达本页</div>
  </div>
</div>`

func handleQRJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	_, _ = w.Write([]byte(qrJS))
}
