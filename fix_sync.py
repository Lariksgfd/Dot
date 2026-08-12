with open('stdlib/sync.dot', 'r', encoding='utf-8') as f:
    content = f.read()

content = content.replace('*void', '*int')

with open('stdlib/sync.dot', 'w', encoding='utf-8') as f:
    f.write(content)
