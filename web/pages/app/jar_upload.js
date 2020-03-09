function $(selector) {
    return document.querySelector(selector);
}

function println(msg) {
    console.info(msg);
    var p = document.createElement('p');
    p.appendChild(document.createTextNode(msg));
    $('#response').appendChild(p);
    p.scrollIntoView()
}

function sendmsg(self, evt) {
    fetch('/jar', {
        method: 'POST',
        body: new FormData(document.myform),
        cache: "no-cache"
    }).then(b => b.text()).then(r => {
        println("msg from server:" + r)
    });
}

let ws;

window.addEventListener('DOMContentLoaded', function () {
    document.getElementById('btnSend').addEventListener('click', sendmsg);

    ws = new WebSocket(`ws://${req_host}/ws`);
    ws.onopen = function(evt) {
        println("OPEN");
    }
    ws.onclose = function(evt) {
        println("CLOSE");
        ws = null;
    }
    ws.onmessage = function(evt) {
        println(evt.data);
    }
    ws.onerror = function(evt) {
        println("ERROR: " + evt.data);
    }

});
window.addEventListener('beforeunload', function () {
    document.getElementById('btnSend').removeEventListener('click', sendmsg);
});