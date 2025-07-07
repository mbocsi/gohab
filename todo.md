# TODO
 - Refactor Web to use a custom server.Client for messages. This client would be a sub module of the web interface, used exclusively for the messages.
 - Reduce the services to only include those not related to messages as these are now handled by a Client.
 - Remove server dependency from web interface. It should only use the (reduced) services + Client.
 - Completely remove the web interface from the server (so remove DI in main.go). Have the web interface be a standalone web server.
 - Register the client in the main.go file (Not attached to a transport)