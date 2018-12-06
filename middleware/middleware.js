// ======= GoReplay Middleware helper =============
// Created by Leonid Bugaev in 2017
//
// For questions use GitHub or support@goreplay.org
//
// GoReplay: https://github.com/buger/goreplay
// Middleware package: https://github.com/buger/goreplay/middleware

var middleware;

function init() {
    var proxy = {
        ch: {},
        on: function(chan, id, cb) {
            if (!cb && id) {
                cb = id;
            } else if (cb && id) {
                chan = chan + "#" + id;
            }

            if (!proxy.ch[chan]) {
                proxy.ch[chan] = [];
            }

            proxy.ch[chan].push({
                created: new Date(),
                cb: cb
            });

            return proxy;
        },

        emit: function(msg, raw) {
            var chanPrefix;

            switch(msg.type) {
                case "1": chanPrefix = "request"; break;
                case "2": chanPrefix = "response"; break;
                case "3": chanPrefix = "replay"; break;
            }

            let resp = msg;

            ["message", chanPrefix, chanPrefix + "#" + msg.ID].forEach(function(chanID, idx){
                if (proxy.ch[chanID]) {
                    proxy.ch[chanID].forEach(function(ch){
                        let r = ch.cb(msg);
                        if (r) resp = r; // If one of callback decided not to send response back, do not override it in global callbacks
                    })
                    
                    // Cleanup Individual message channels to avoid memory leaks
                    if (idx == 2) {
                        delete proxy.ch[chanID]
                    }
                }
            })

            if (resp) {
              process.stdout.write(`${resp.rawMeta.toString('hex')}${Buffer.from("\n").toString("hex")}${resp.http.toString('hex')}\n`)
            }
        }
    }

    // Clean up old messaged ID specific channels if they are older then 60s
    let gc = function(gcTime){
        let now = new Date();
        for (k in proxy.ch) {
            if (k.indexOf("#") == -1) continue;

            proxy.ch[k] = proxy.ch[k].filter(function(ch){
                return (now - ch.created) < gcTime
            })

            if (proxy.ch[k].length == 0) {
                delete proxy.ch[k]
            }
        }
    }
    proxy.gc = gc

    setInterval(function(){
        gc(10 * 1000)
    }, 1000);

    const readline = require('readline');
    const rl = readline.createInterface({
          input: process.stdin
    });

    rl.on('line', function(line) {
        let msg = parseMessage(line)
        if (msg) {
            proxy.emit(msg, line)
        }
    });

    middleware = proxy;

    return proxy;
}


function parseMessage(msg) {
    try {
        let payload = Buffer.from(msg, "hex");
        let metaPos = payload.indexOf("\n");
        let meta = payload.slice(0, metaPos);
        let metaArr = meta.toString("ascii").split(" ");
        let pType = metaArr[0];
        let pID = metaArr[1];
        let raw = payload.slice(metaPos + 1, payload.length);

        return {
            type: pType,
            ID: pID,
            rawMeta: meta,
            meta: metaArr,
            http: raw
        }
    } catch(e) {
        fail(`Error while parsing incoming request: ${msg}`)
    }
}

// Used to compare values from original and replayed responses
// Accepts request id, regexp pattern for searching the compared value (should include capture group), and callback which returns both original and replayed matched value.
// Example: 
//
//   // Compare HTTP headers for response and replayed response, and map values
//   let tokMap = {};
//
//   gor.on("request", function(req) {
//     let tok = gor.httpHeader(req.http, "Auth-Token");
//     if (tok && tokMap[tok]) {
//       req.http = gor.setHttpHeader(req.http, "Auth-Token", tokMap[tok]) 
//     }
//
//     gor.searchResponses(req.ID, "X-Set-Token: (\w+)$", function(respTok, replTok) {
//       tokMap[respTok] = replTok;
//     })
//
//     return req;
//   })
//
function searchResponses(id, searchPattern, callback) {
    let re = new RegExp(searchPattern);

    // Using regexp require converting buffer to string
    // Before converting to string we can use initial `Buffer.indexOf` check
    let indexPattern = searchPattern.split("(")[0];

    if (!indexPattern) {
        console.error("Search regexp should include capture group, pointing to the value: `prefix-(.*)`")
        return
    }

    middleware.on("response", id, function(resp){
        if (resp.http.indexOf(indexPattern) == -1) {
            callback()
            return resp
        }

        let respMatch = resp.http.toString('utf-8').match(re);
        if (!respMatch) {
            callback()
            return resp
        }

        middleware.on("replay", id, function(repl) {
            if (repl.http.indexOf(indexPattern) == -1) {
                callback(respMatch[1]);
                return repl;
            }

            let replMatch = repl.http.toString('utf-8').match(re);

            if (!replMatch) {
                callback(respMatch[1]);
                return repl;
            }
        
            callback(respMatch[1], replMatch[1]);
            
            return repl;
        })

        return resp;
    })
}


// =========== HTTP parsing =================

// Example HTTP payload record (including hidden characters):
//
//  POST / HTTP/1.1\r\n
//  User-Agent: Node\r\n
//  Content-Length: 5\r\n
//  \r\n
//  hello

function httpMethod(payload) {
    var pEnd = payload.indexOf(' ');
    return payload.slice(0, pEnd).toString("ascii");
}

function httpPath(payload) {
    var pStart = payload.indexOf(' ') + 1;
    var pEnd = payload.indexOf(' ', pStart);
    return payload.slice(pStart, pEnd).toString("ascii");
}

function setHttpPath(payload, newPath) {
    var pStart = payload.indexOf(' ') + 1;
    var pEnd = payload.indexOf(' ', pStart);

    return Buffer.concat([payload.slice(0, pStart), Buffer.from(newPath), payload.slice(pEnd, payload.length)])
}

function httpPathParam(payload, name) {
    let path = httpPath(payload);
    let re = new RegExp(name + "=([^&$]+)");
    let match = path.match(re);

    if (match) return decodeURI(match[1]);
}

function setHttpPathParam(payload, name, value) {
    let path = httpPath(payload);
    let re = new RegExp(name + "=([^&$]+)");
    let newPath = path.replace(re, name + "=" + encodeURI(value));
    
    // If we should add new param instead
    if (newPath == path) {
        if (newPath.indexOf("?") == -1) {
            newPath += "?"
        } else {
            newPath += "&"
        }

        newPath += name + "=" + encodeURI(value);
    }

    return setHttpPath(payload, newPath)
}

// HTTP response have status code in same position as `path` for requests
function httpStatus(payload) {
    return httpPath(payload);
}

function setHttpStatus(payload, newStatus) {
    return setHttpPath(payload, newStatus);
}

function httpHeaders(payload) {
    var httpHeaderString = payload.slice(0,payload.indexOf("\r\n\r\n") + 4).toString().split("\n").slice(1);
    var headers = {};

    for (var item in httpHeaderString) {
        var parts = httpHeaderString[item].split(":");

        if (parts.length > 1) {
            headers[parts[0]] = parts.slice(1).join(":").trim();    
        }
    }

    return headers;
}

function httpHeader(payload, name) {
    var currentLine = 0;
    var i = 0;
    var header = { start: -1, end: -1, valueStart: -1 }
    var nameBuf = Buffer.from(name);
    var nameBufLower = Buffer.from(name.toLowerCase());

    while(c = payload[i]) {
        if (c == 13) { // new line "\n"
            currentLine++;
            i++
            header.end = i

            if (currentLine > 0 && header.start > 0 && header.valueStart > 0) {
                if (nameBuf.compare(payload, header.start, header.valueStart - 1) == 0 ||
                    nameBufLower.compare(payload, header.start, header.valueStart - 1) == 0) { // ensure that headers are not case sensitive
                    header.value = payload.slice(header.valueStart, header.end - 1).toString("utf-8").trim();
                    header.name = payload.slice(header.start, header.valueStart - 1).toString("utf-8");
                    return header
                }
            }

            header.start = -1
            header.valueStart = -1
            continue;
        } else if (c == 10) { // "\r"
            i++
            continue;
        } else if (c == 58) { // ":" Header/value separator symbol
            if (header.valueStart == -1) {
                header.valueStart = i + 1;
                i++
                continue;
            }
        }

        if (header.start == -1) header.start = i;

        i++
    }

    return
}

function setHttpHeader(payload, name, value) {
    let header = httpHeader(payload, name);
    if (!header) {
        let headerStart = payload.indexOf(13) + 1;
        return Buffer.concat([payload.slice(0, headerStart + 1), Buffer.from(name + ": " + value + "\r\n"), payload.slice(headerStart + 1, payload.length)])
    } else {
        return Buffer.concat([payload.slice(0, header.valueStart), Buffer.from(" " + value + "\r\n"), payload.slice(header.end + 1, payload.length)])
    }
}

function deleteHttpHeader(payload, name) {
	let header = httpHeader(payload, name);

    if (header) {
        return Buffer.concat([payload.slice(0, header.start), payload.slice(header.end+1, payload.length)])
    }

	return payload
}

function httpBody(payload) {
    return payload.slice(payload.indexOf("\r\n\r\n") + 4, payload.length);
}

function setHttpBody(payload, newBody) {
    let p = setHttpHeader(payload, "Content-Length", newBody.length)
    let headerEnd = p.indexOf("\r\n\r\n") + 4;
    return Buffer.concat([p.slice(0, headerEnd), newBody])
}

function httpBodyParam(payload, name) {
    let body = httpBody(payload);
    let re = new RegExp(name + "=([^&$]+)");
    if (body.indexOf(name + "=") != -1) {
        let param = body.toString('utf-8').match(re);
        if (param) {
            return decodeURI(param[1]);
        }
    }
}

function setHttpBodyParam(payload, name, value) {
    let body = httpBody(payload);
    let re = new RegExp(name + "=([^&$]+)");

    let newBody = body.toString('utf-8');

    if (newBody.indexOf(name + "=") != -1 ) {
        newBody = newBody.replace(re, name + "=" + encodeURI(value));
    } else {
        if (newBody.indexOf("=") != -1) {
            newBody += "&";
        }
        newBody += name + "=" + value;
    }
    
    return setHttpBody(payload, Buffer.from(newBody));
}

function setHttpCookie(payload, name, value) {
    let h = httpHeader(payload, "Cookie");
    let cookie = h ? h.value : "";
    let cookies = cookie.split("; ").filter(function(v){ return v.indexOf(name + "=") != 0 })
    cookies.push(name + "=" + value)
    return setHttpHeader(payload, "Cookie", cookies.join("; "))
}

function httpCookie(payload, name) {
    let h = httpHeader(payload, "Cookie");
    let cookie = h ? h.value : "";
    let value;
    let cookies = cookie.split("; ").forEach(function(v){
        if (v.indexOf(name + "=") == 0) {
            value = v.substr(name.length + 1);
        }
    })
    return value;
}

module.exports = {
    init: init,
    on: function(){ return middleware.on.apply(this, arguments) },
    parseMessage: parseMessage,
    searchResponses: searchResponses,
    httpPath: httpPath,
    httpMethod: httpMethod,
    setHttpPath: setHttpPath,
    httpPathParam: httpPathParam,
    setHttpPathParam: setHttpPathParam,
    httpStatus: httpStatus,
    setHttpStatus: setHttpStatus,
    httpHeader: httpHeader,
    setHttpHeader: setHttpHeader,
    deleteHttpHeader: deleteHttpHeader,
    httpBody: httpBody,
    setHttpBody: setHttpBody,
    httpBodyParam: httpBodyParam,
    setHttpBodyParam: setHttpBodyParam,
    httpCookie: httpCookie,
    setHttpCookie: setHttpCookie,
    test: testRunner,
    benchmark: testBenchmark,
    httpHeaders: httpHeaders
}


// =========== Tests ==============

function testRunner(){
    ["init", "parseMessage", "httpMethod", "httpPath", "setHttpHeader", "deleteHttpHeader", "httpPathParam", "httpHeader", "httpBody", "setHttpBody", "httpBodyParam", "httpCookie", "setHttpCookie", "httpHeaders"].forEach(function(t){
        console.log(`====== Start ${t} =======`)
        eval(`TEST_${t}()`)
        console.log(`====== End ${t} =======`)
    })
}

function testBenchmark(){
    const child_process = require('child_process');

    let gor = init();
    gor.on("message", function(){
    });

    gor.on("request", function(){
    });

    for (var i = 0; i<256; i++) {
        let req = parseMessage(Buffer.from("1 2 3\nGET / HTTP/1.1\r\n\r\n").toString('hex'));
        req.ID = +Date.now()
        gor.emit(req);

        gor.on("request", req.ID+"", function(){
            gor.on("response", req.ID+"", function(){
            })
        })

        if ( i % 3 == 0 ) {
            let resp = parseMessage(Buffer.from("2 2 3\nHTTP/1.1 200 OK\r\n\r\n").toString('hex'));
            resp.ID = req.ID
            gor.emit(resp);
        }
    }
    
    child_process.execSync("sleep 0.01");

    gor.gc(1) 
    
    fail(JSON.stringify(gor.ch))
}

// Just print in red color
function fail(message) {
    console.error("\x1b[31m[MIDDLEWARE] %s\x1b[0m", message)
}

function log(message) {
    console.error(message)
}

function TEST_init() {
    const child_process = require('child_process');

    let received = 0;
    let gor = init();
    gor.on("message", function(){
        received++; // should be called 3 times for for every request
    });

    gor.on("request", function(){
        received++; // should be called 1 time only for request
    });

    gor.on("response", "2", function(){
        received++; // should be called 1 time only for specific response
    })

    if (Object.keys(gor.ch).length != 3) {
        return fail("Should create 3 channels");
    }

    let req = parseMessage(Buffer.from("1 2 3\nGET / HTTP/1.1\r\n\r\n").toString('hex'));
    let resp = parseMessage(Buffer.from("2 2 3\nHTTP/1.1 200 OK\r\n\r\n").toString('hex'));
    let resp2 = parseMessage(Buffer.from("2 3 3\nHTTP/1.1 200 OK\r\n\r\n").toString('hex'));
    gor.emit(req);
    gor.emit(resp);
    gor.emit(resp2);

    child_process.execSync("sleep 0.01");

    if (received != 5) {
        fail(`Should receive 5 messages: ${received}`);
    }
}

function TEST_parseMessage() {
    const exampleMessage = Buffer.from("1 2 3\nGET / HTTP/1.1\r\n\r\n").toString('hex')
    let msg = parseMessage(exampleMessage)
    let expected = { type: '1', ID: '2', meta: ["1", "2", "3"], http: Buffer.from("GET / HTTP/1.1\r\n\r\n") }

    Object.keys(expected).forEach(function(k){
        if (msg[k].toString() != expected[k].toString()) {
            fail(`${k}: '${expected[k]}' != '${msg[k]}'`)
        }
    })
}

function TEST_httpPath() {
    const examplePayload = "GET /test HTTP/1.1\r\n\r\n";

    let payload = Buffer.from(examplePayload);
    let path = httpPath(payload);

    if (path != "/test") {
        return fail(`Path '${patj}' != '/test'`)
    }

    let newPayload = setHttpPath(payload, '/')
    if (newPayload.toString() != "GET / HTTP/1.1\r\n\r\n") {
        return fail(`Malformed payload '${newPayload}'`)
    }

    newPayload = setHttpPath(payload, '/bigger')
    if (newPayload.toString() != "GET /bigger HTTP/1.1\r\n\r\n") {
        return fail(`Malformed payload '${newPayload}'`)
    }
}

function TEST_httpMethod() {
    const examplePayload = "GET /test HTTP/1.1\r\n\r\n";

    let payload = Buffer.from(examplePayload);
    let method = httpMethod(payload);

    if (method != "GET") {
        return fail(`Path '${method}' != 'GET'`)
    }
}


function TEST_httpPathParam() {
    let p = Buffer.from("GET / HTTP/1.1\r\n\r\n");

    if (httpPathParam(p, "test")) {
        return fail("Should not found param")
    }

    p = setHttpPathParam(p, "test", "123");
    if (httpPath(p) != "/?test=123") {
        return fail("Should set first param: " + httpPath(p));
    }

    if (httpPathParam(p, "test") != "123") {
        return fail("Should get first param: " + httpPathParam(p, "test"));
    }

    p = setHttpPathParam(p, "qwer", "ty");
    if (httpPath(p) != "/?test=123&qwer=ty") {
        return fail("Should set second param: " + httpPath(p));
    }

    p = setHttpPathParam(p, "test", "4321");
    if (httpPath(p) != "/?test=4321&qwer=ty") {
        return fail("Should update first param: " + httpPath(p));
    }

    if (httpPathParam(p, "test") != "4321") {
        return fail("Should update first param: " + httpPath(p));
    }
}

function TEST_httpBodyParam() {
    let p = Buffer.from("POST / HTTP/1.1\r\n\r\n");

    if (httpBodyParam(p, "test")) {
        return fail("Should not found param")
    }

    p = setHttpBodyParam(p, "test", "123");
    if (httpBody(p).toString() != "test=123") {
        return fail("Should set first param: " + httpBody(p).toString());
    }

    if (httpBodyParam(p, "test") != "123") {
        return fail("Should get first param: " + httpBodyParam(p, "test"));
    }

    p = setHttpBodyParam(p, "qwer", "ty");
    if (httpBody(p).toString() != "test=123&qwer=ty") {
        return fail("Should set second param: " + httpBody(p).toString());
    }

    p = setHttpBodyParam(p, "test", "4321");
    if (httpBody(p).toString() != "test=4321&qwer=ty") {
        return fail("Should update first param: " + httpBody(p).toString());
    }

    if (httpBodyParam(p, "test") != "4321") {
        return fail("Should update first param: " + httpBody(p).toString());
    }
}

function TEST_httpHeader() {
    const examplePayload = "GET / HTTP/1.1\r\nHost: localhost:3000\r\nUser-Agent: Node\r\nContent-Length:5\r\n\r\nhello";

    let expected = {"Host": "localhost:3000", "User-Agent": "Node", "Content-Length": "5"}

    Object.keys(expected).forEach(function(name){
        let payload = Buffer.from(examplePayload);
        let header = httpHeader(payload, name);
        if (!header) {
            fail(`Header not found. Was looking for: ${name}`)
        }
        if (header && header.value != expected[name]) {
            fail(`${name}: '${expected[name]}' != '${header.value}'`)
        }
    })
}


function TEST_setHttpHeader() {
    const examplePayload = "GET / HTTP/1.1\r\nUser-Agent: Node\r\nContent-Length: 5\r\n\r\nhello";

    // Modify existing header
    ["", "1", "Long test header"].forEach(function(ua){
        let expected = `GET / HTTP/1.1\r\nUser-Agent: ${ua}\r\nContent-Length: 5\r\n\r\nhello`;
        let p = Buffer.from(examplePayload);
        p = setHttpHeader(p, "User-Agent", ua);
        if (p != expected) {
            console.error(`setHeader failed, expected User-Agent value: ${ua}.\n${p}`)
        }
    })

    // Adding new header
    let expected = `GET / HTTP/1.1\r\nX-Test: test\r\nUser-Agent: Node\r\nContent-Length: 5\r\n\r\nhello`;
    let p = Buffer.from(examplePayload);
    p = setHttpHeader(p, "X-Test", "test");
    if (p != expected) {
        console.error(`setHeader failed, expected new header 'X-Test' header: ${p}`)
    }
}

function TEST_deleteHttpHeader() {
    const examplePayload = "GET / HTTP/1.1\r\nUser-Agent: Node\r\nContent-Length: 5\r\n\r\nhello";

    // Adding new header
    let expected = `GET / HTTP/1.1\r\nContent-Length: 5\r\n\r\nhello`;
    let p = Buffer.from(examplePayload);
    p = deleteHttpHeader(p, "User-Agent", "test");
    if (p != expected) {
        console.error(`setHeader failed, expected delete header 'User-Agent' header: ${p}`)
    }
}

function TEST_httpBody() {
    const examplePayload = "GET / HTTP/1.1\r\nUser-Agent: Node\r\nContent-Length: 5\r\n\r\nhello";
    let body = httpBody(Buffer.from(examplePayload));
    if (body != "hello") {
        fail(`'${body}' != 'hello'`)
    }
}

function TEST_setHttpBody() {
    const examplePayload = "GET / HTTP/1.1\r\nUser-Agent: Node\r\nContent-Length: 5\r\n\r\nhello";
    let p = setHttpBody(Buffer.from(examplePayload), Buffer.from("hello, world!"));

    if (p != "GET / HTTP/1.1\r\nUser-Agent: Node\r\nContent-Length: 13\r\n\r\nhello, world!") {
        fail(`Wrong body: '${p}'`)
    }
}

function TEST_httpCookie() {
    const examplePayload = "GET / HTTP/1.1\r\nCookie: a=b; test=zxc\r\n\r\n";
    let c = httpCookie(Buffer.from(examplePayload), "test");
    if (c != "zxc") {
        return fail(`Should get cookie: ${c}`);
    }

    c = httpCookie(Buffer.from(examplePayload), "nope");
    if (c != null) {
        return fail(`Should not find cookie: ${c}`);
    }
}

function TEST_setHttpCookie() {
    const examplePayload = "GET / HTTP/1.1\r\nCookie: a=b; test=zxc\r\n\r\n";
    let p = setHttpCookie(Buffer.from(examplePayload), "test", "1");
    if (p != "GET / HTTP/1.1\r\nCookie: a=b; test=1\r\n\r\n") {
        return fail(`Should update cookie: ${p}`)
    }

    p = setHttpCookie(Buffer.from(examplePayload), "new", "one");
    if (p != "GET / HTTP/1.1\r\nCookie: a=b; test=zxc; new=one\r\n\r\n") {
        return fail(`Should add new cookie: ${p}`)
    }
}

function TEST_httpHeaders() {
    const examplePayload = "GET / HTTP/1.1\r\nHost: localhost:3000\r\nUser-Agent: Node\r\nContent-Length:5\r\n\r\nhello";

    let expectedHeaders = {"Host": "localhost:3000", "User-Agent": "Node", "Content-Length": "5"}
    let payload = Buffer.from(examplePayload);
    let headers = httpHeaders(payload);

    ["Host", "User-Agent", "Content-Length"].forEach(function(header){
        let actual = headers[header];
        let expected = expectedHeaders[header];

        if (!actual) {
            fail(`${header} Header was not found`);
        }

        if (actual != expected) {
            fail(`${header} Header not Equal to Expected: ${expected} was ${actual}`);
        }

    })
}
