with open('stdlib/testing.dot', 'r', encoding='utf-8') as f:
    content = f.read()

content = content.replace('fn assert_eq[T]', 'fn assert_eq[T Comparable]')
content = content.replace('fn assert_ne[T]', 'fn assert_ne[T Comparable]')

with open('stdlib/testing.dot', 'w', encoding='utf-8') as f:
    f.write(content)
