import re

with open('stdlib/http.dot', 'r', encoding='utf-8') as f:
    content = f.read()

content = content.replace('import "net"', 'import "std.net" as net')
content = content.replace('import "fmt"', 'import "std.fmt" as fmt')
content = content.replace('let fd = ', 'fd = ')
content = content.replace('let req = ', 'req = ')
content = content.replace('let body = ', 'body = ')

with open('stdlib/http.dot', 'w', encoding='utf-8') as f:
    f.write(content)
