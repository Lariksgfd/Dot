import re

with open('stdlib/fmt.dot', 'r', encoding='utf-8') as f:
    content = f.read()

# Replace all "something" with 'something' for characters in comparisons and assignments if it's 1 char
# It's safer to just replace them manually.

content = content.replace('template[i] == "{"', "template[i] == '{'")
content = content.replace('template[i + 1] == "}"', "template[i + 1] == '}'")
content = content.replace('ch != " "', "ch != ' '")
content = content.replace('ch != "\\t"', "ch != '\\t'")
content = content.replace('ch != "\\n"', "ch != '\\n'")
content = content.replace('ch != "\\r"', "ch != '\\r'")

content = content.replace('ch >= "a"', "ch >= 'a'")
content = content.replace('ch <= "z"', "ch <= 'z'")
content = content.replace('ch >= "A"', "ch >= 'A'")
content = content.replace('ch <= "Z"', "ch <= 'Z'")

content = content.replace('fn char_upper(ch string) -> string', 'fn char_upper(ch rune) -> rune')
content = content.replace('fn char_lower(ch string) -> string', 'fn char_lower(ch rune) -> rune')

# The bodies of char_upper and char_lower need fixing: if ch == "a" { return "A" } -> if ch == 'a' { return 'A' }
def fix_char_func(match):
    return match.group(0).replace('"', "'")

content = re.sub(r'fn char_upper.*?^}', fix_char_func, content, flags=re.MULTILINE|re.DOTALL)
content = re.sub(r'fn char_lower.*?^}', fix_char_func, content, flags=re.MULTILINE|re.DOTALL)

with open('stdlib/fmt.dot', 'w', encoding='utf-8') as f:
    f.write(content)
