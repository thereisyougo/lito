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

function cleanmsg(self, evt) {
    $('#message').value = '';
}

let ws;

function trip() {
    let val = $('#message').value;
    if (val !== '') {
        ws.send(val);
    }
}

window.addEventListener('DOMContentLoaded', function () {
    $('#btnSend').addEventListener('click', sendmsg);
    $('#sendMsg').addEventListener('click', trip);
    $('#cleanMsg').addEventListener('click', cleanmsg);

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
    $('#btnSend').removeEventListener('click', sendmsg);
    $('#sendMsg').removeEventListener('click', trip)
    $('#cleanMsg').removeEventListener('click', cleanmsg);
});
