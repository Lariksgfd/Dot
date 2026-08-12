with open('stdlib/process.dot', 'r', encoding='utf-8') as f:
    content = f.read()

content = content.replace('extern struct', 'struct')
content = content.replace('extern fn', '@extern("C")\nfn')

with open('stdlib/process.dot', 'w', encoding='utf-8') as f:
    f.write(content)
