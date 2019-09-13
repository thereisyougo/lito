function sendmsg(self, evt) {
    fetch('/jar', {
        method: 'POST',
        body: new FormData(document.myform),
        cache: "no-cache"
    }).then(b => b.text()).then(r => {
        console.info("msg from server:" + r)
    });
}

let ws;

window.addEventListener('DOMContentLoaded', function () {
    document.getElementById('btnSend').addEventListener('click', sendmsg);

    ws = new WebSocket(`ws://${req_host}/ws`);
    ws.onopen = function(evt) {
        console.info("OPEN");
    }
    ws.onclose = function(evt) {
        console.info("CLOSE");
        ws = null;
    }
    ws.onmessage = function(evt) {
        console.info(evt.data);
    }
    ws.onerror = function(evt) {
        console.info("ERROR: " + evt.data);
    }

});
window.addEventListener('beforeunload', function () {
    document.getElementById('btnSend').removeEventListener('click', sendmsg);
});