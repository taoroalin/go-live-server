if ('WebSocket' in window) {
  var protocol = window.location.protocol === 'https:' ? 'wss://' : 'ws://';
  var address = protocol + window.location.host + window.location.pathname + '/ws';
  var socket = new WebSocket(address);
  socket.onmessage = function (msg) {
    if (msg.data == 'reload') window.location.reload();
  };
  console.log('Live reload enabled.');
}