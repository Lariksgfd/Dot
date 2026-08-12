with open('stdlib/net.dot', 'r', encoding='utf-8') as f:
    content = f.read()

content = 'module net\n\n' + content

with open('stdlib/net.dot', 'w', encoding='utf-8') as f:
    f.write(content)
