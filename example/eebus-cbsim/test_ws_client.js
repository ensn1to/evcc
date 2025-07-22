const WebSocket = require('ws');

console.log('Connecting to WebSocket server...');
const ws = new WebSocket('ws://localhost:7071/ws');

ws.on('open', function open() {
    console.log('Connected to WebSocket server');
    
    // Send GetEntityList request
    console.log('Sending GetEntityList request...');
    ws.send(JSON.stringify({Type: 4}));
    
    // Send GetAllData request
    console.log('Sending GetAllData request...');
    ws.send(JSON.stringify({Type: 5}));
});

ws.on('message', function message(data) {
    console.log('Received:', data.toString());
    try {
        const message = JSON.parse(data.toString());
        if (message.Type === 1) { // QRCode
            console.log('QR Code received:', message.Text);
        }
    } catch (e) {
        console.log('Error parsing message:', e);
    }
});

ws.on('error', function error(err) {
    console.log('WebSocket error:', err);
});

ws.on('close', function close() {
    console.log('WebSocket connection closed');
});

// Keep the process running for a few seconds
setTimeout(() => {
    ws.close();
    process.exit(0);
}, 5000);
