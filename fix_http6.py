import re

with open('stdlib/http.dot', 'r', encoding='utf-8') as f:
    content = f.read()

content = content.replace('import std.fmt\n', '')

with open('stdlib/http.dot', 'w', encoding='utf-8') as f:
    f.write(content)
