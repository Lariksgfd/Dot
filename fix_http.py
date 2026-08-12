import re

with open('stdlib/http.dot', 'r', encoding='utf-8') as f:
    content = f.read()

content = content.replace('fn (s *HttpServer) listen() -> int {', 'impl HttpServer {\n    fn listen(mut self) -> int {')
content = content.replace('s.fd', 'self.fd')
content = content.replace('s.port', 'self.port')
content = content.replace('fn (s *HttpServer) accept() -> int {', '    fn accept(self) -> int {')
# Add missing closing brace for the impl block
content += '\n}\n'

with open('stdlib/http.dot', 'w', encoding='utf-8') as f:
    f.write(content)
