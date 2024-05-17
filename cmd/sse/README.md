# SSE

Server-side events have limits and run in trouble when you open lots of tabs. As a workaround you can move the SSE code into a service worker and broadcast the messages to the tabs. Not all browsers support SSE in a service worker (Firefox doesn't) so as a workaround you can use Fetch and a ReadableStream to implement SSE support yourself. That's what this code does.

Bear in mind that you have to test service workers on localhost or over https or the browser will report that service workers aren't present.
