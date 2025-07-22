#!/bin/bash

# Test script to verify graceful shutdown functionality

echo "Testing graceful shutdown of controlbox..."

# Start the controlbox in background with dummy remoteSki
echo "Starting controlbox on port 8080..."
./controlbox 8080 dummy_ski cert.pem key.pem &
CONTROLBOX_PID=$!

# Wait a moment for the server to start
sleep 2

# Check if the process is running
if kill -0 $CONTROLBOX_PID 2>/dev/null; then
    echo "✓ Controlbox started successfully (PID: $CONTROLBOX_PID)"
else
    echo "✗ Failed to start controlbox"
    exit 1
fi

# Test HTTP server is responding
echo "Testing HTTP server..."
if curl -s http://localhost:7071 > /dev/null; then
    echo "✓ HTTP server is responding"
else
    echo "✗ HTTP server is not responding"
fi

# Send SIGINT to test graceful shutdown
echo "Sending SIGINT to test graceful shutdown..."
kill -INT $CONTROLBOX_PID

# Wait for the process to terminate gracefully
sleep 3

# Check if process terminated
if kill -0 $CONTROLBOX_PID 2>/dev/null; then
    echo "✗ Process did not terminate gracefully, forcing kill..."
    kill -KILL $CONTROLBOX_PID
    exit 1
else
    echo "✓ Process terminated gracefully"
fi

echo "✓ Graceful shutdown test passed!"
