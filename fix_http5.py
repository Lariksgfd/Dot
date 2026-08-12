import re

with open('stdlib/http.dot', 'r', encoding='utf-8') as f:
    content = f.read()

content = content.replace('import net', 'import std.net')
content = content.replace('import fmt', 'import std.fmt')

with open('stdlib/http.dot', 'w', encoding='utf-8') as f:
    f.write(content)
