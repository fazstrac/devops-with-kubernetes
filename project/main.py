import os
import cherrypy

PORT = int(os.environ.get("DWK_PORT", 8080))

class DWKApp(object):
    @cherrypy.expose
    def index(self):
        return f"Server started in port {PORT}"

if __name__ == '__main__':
    print(f"Server started in port {PORT}")
    cherrypy.config.update({
        'server.socket_port': PORT,
        'server.socket_host': '0.0.0.0',
        'environment': 'production',
    })
    cherrypy.quickstart(DWKApp())