import re
import os

def fix_file(filepath):
    with open(filepath, 'r', encoding='utf-8') as f:
        content = f.read()

    # Revert the bad body body back to body: body
    content = content.replace('body body}', 'body: body}')
    content = content.replace('port port, fd', 'port: port, fd')
    content = content.replace('args: []string', 'args []string')
    content = content.replace('module std.process', '// module process')
    
    with open(filepath, 'w', encoding='utf-8') as f:
        f.write(content)

for f in ['stdlib/http.dot', 'stdlib/net.dot', 'stdlib/process.dot']:
    fix_file(f)
