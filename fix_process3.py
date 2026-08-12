with open('stdlib/process.dot', 'r', encoding='utf-8') as f:
    content = f.read()

content = content.replace('args.len()', 'args.len')

with open('stdlib/process.dot', 'w', encoding='utf-8') as f:
    f.write(content)
