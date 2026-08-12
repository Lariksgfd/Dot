import re
with open('stdlib/chan.dot', 'r', encoding='utf-8') as f:
    content = f.read()

content = content.replace('*void', '*int')
content = content.replace('v any', 'v int')
content = content.replace('-> any', '-> int')

with open('stdlib/chan.dot', 'w', encoding='utf-8') as f:
    f.write(content)
