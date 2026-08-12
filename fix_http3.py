import re

with open('stdlib/http.dot', 'r', encoding='utf-8') as f:
    content = f.read()

content = content.replace('import "std.net" as net', 'import std.net as net')
content = content.replace('import "std.fmt" as fmt', 'import std.fmt as fmt')

with open('stdlib/http.dot', 'w', encoding='utf-8') as f:
    f.write(content)
