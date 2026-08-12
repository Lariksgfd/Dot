import re
with open('stdlib/testing.dot', 'r', encoding='utf-8') as f:
    content = f.read()

content = re.sub(r'fn assert_eq\[T: Comparable\].*?^}', '', content, flags=re.MULTILINE|re.DOTALL)
content = re.sub(r'fn assert_ne\[T: Comparable\].*?^}', '', content, flags=re.MULTILINE|re.DOTALL)

with open('stdlib/testing.dot', 'w', encoding='utf-8') as f:
    f.write(content)
