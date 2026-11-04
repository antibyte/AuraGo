# ascii — ASCII code table (0–127) / ASCII-Codetabelle (0–127)

Kind: definitional
Guide-value note needed: no

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| dec | code in decimal | Code dezimal | – | yes |
| hex | code in hexadecimal (two digits) | Code hexadezimal | – | yes (hex) |
| char | the printable character, or the standard abbreviation of a control character; SP = space (code 32) | druckbares Zeichen bzw. Kürzel des Steuerzeichens; SP = Leerzeichen | – | no |
| kind | `control` or `graphic` (printable) | `control` = Steuerzeichen, `graphic` = druckbares Zeichen | – | no |
| rfc20_class | control-character class from RFC 20: CC = communication control, FE = format effector, IS = information separator (empty = none given) | Klasse laut RFC 20: CC = Übertragungssteuerung, FE = Formatsteuerung, IS = Informationstrenner | – | no |
| ctrl | caret notation of control characters (code XOR 0x40, i.e. Ctrl+key; DEL = ^?) | Caret-Schreibweise (Strg+Taste) | – | no |
| name | name "en\|de" (en = RFC 20 legend; modern name added in brackets where the 1969 name is archaic) | Name "en\|de" | – | no |

## Rows
```csv
dec,hex,char,kind,rfc20_class,ctrl,name
0,00,NUL,control,,^@,Null|Nullzeichen
1,01,SOH,control,CC,^A,Start of Heading|Anfang des Kopfes
2,02,STX,control,CC,^B,Start of Text|Anfang des Textes
3,03,ETX,control,CC,^C,End of Text|Ende des Textes
4,04,EOT,control,CC,^D,End of Transmission|Ende der Übertragung
5,05,ENQ,control,CC,^E,Enquiry|Stationsaufforderung
6,06,ACK,control,CC,^F,Acknowledge|Positive Rückmeldung
7,07,BEL,control,,^G,Bell (audible or attention signal)|Klingel
8,08,BS,control,FE,^H,Backspace|Rückwärtsschritt
9,09,HT,control,FE,^I,Horizontal Tabulation|Zeichen-Tabulator
10,0A,LF,control,FE,^J,Line Feed|Zeilenschritt (Zeilenvorschub)
11,0B,VT,control,FE,^K,Vertical Tabulation|Zeilentabulatorzeichen
12,0C,FF,control,FE,^L,Form Feed|Formularvorschub
13,0D,CR,control,FE,^M,Carriage Return|Rückführen (Wagenrücklauf)
14,0E,SO,control,,^N,Shift Out|Dauerumschaltung
15,0F,SI,control,,^O,Shift In|Rückschaltung
16,10,DLE,control,CC,^P,Data Link Escape|Datenübertragungsumschaltung
17,11,DC1,control,,^Q,Device Control 1|Gerätesteuerzeichen 1
18,12,DC2,control,,^R,Device Control 2|Gerätesteuerzeichen 2
19,13,DC3,control,,^S,Device Control 3|Gerätesteuerzeichen 3
20,14,DC4,control,,^T,Device Control 4 (Stop)|Gerätesteuerzeichen 4
21,15,NAK,control,CC,^U,Negative Acknowledge|Negative Rückmeldung
22,16,SYN,control,CC,^V,Synchronous Idle|Synchronisierung
23,17,ETB,control,CC,^W,End of Transmission Block|Ende des Datenübertragungsblockes
24,18,CAN,control,,^X,Cancel|Ungültig
25,19,EM,control,,^Y,End of Medium|Ende der Aufzeichnung
26,1A,SUB,control,,^Z,Substitute|Substitution
27,1B,ESC,control,,^[,Escape|Escape
28,1C,FS,control,IS,^\,File Separator|Hauptgruppen-Trennzeichen (Informations-Trennzeichen Vier)
29,1D,GS,control,IS,^],Group Separator|Gruppen-Trennzeichen (Informations-Trennzeichen Drei)
30,1E,RS,control,IS,^^,Record Separator|Untergruppen-Trennzeichen (Informations-Trennzeichen Zwei)
31,1F,US,control,IS,^_,Unit Separator|Teilgruppen-Trennzeichen (Informations-Trennzeichen Eins)
32,20,SP,graphic,,,Space|Leerzeichen
33,21,!,graphic,,,Exclamation Point|Ausrufezeichen
34,22,"""",graphic,,,Quotation Marks|Anführungszeichen
35,23,#,graphic,,,Number Sign|Nummernzeichen (Raute)
36,24,$,graphic,,,Dollar Sign|Dollarzeichen
37,25,%,graphic,,,Percent|Prozentzeichen
38,26,&,graphic,,,Ampersand|Und-Zeichen (Et-Zeichen)
39,27,',graphic,,,Apostrophe|Apostroph
40,28,(,graphic,,,Opening Parenthesis|runde Klammer auf
41,29,),graphic,,,Closing Parenthesis|runde Klammer zu
42,2A,*,graphic,,,Asterisk|Sternchen
43,2B,+,graphic,,,Plus|Pluszeichen
44,2C,",",graphic,,,Comma|Komma
45,2D,-,graphic,,,Hyphen (Minus)|Bindestrich (Minus)
46,2E,.,graphic,,,Period (Decimal Point)|Punkt (Dezimalpunkt)
47,2F,/,graphic,,,Slant (slash)|Schrägstrich
48,30,0,graphic,,,Digit 0|Ziffer 0
49,31,1,graphic,,,Digit 1|Ziffer 1
50,32,2,graphic,,,Digit 2|Ziffer 2
51,33,3,graphic,,,Digit 3|Ziffer 3
52,34,4,graphic,,,Digit 4|Ziffer 4
53,35,5,graphic,,,Digit 5|Ziffer 5
54,36,6,graphic,,,Digit 6|Ziffer 6
55,37,7,graphic,,,Digit 7|Ziffer 7
56,38,8,graphic,,,Digit 8|Ziffer 8
57,39,9,graphic,,,Digit 9|Ziffer 9
58,3A,:,graphic,,,Colon|Doppelpunkt
59,3B,;,graphic,,,Semicolon|Semikolon
60,3C,<,graphic,,,Less Than|Kleiner-als-Zeichen
61,3D,=,graphic,,,Equals|Gleichheitszeichen
62,3E,>,graphic,,,Greater Than|Größer-als-Zeichen
63,3F,?,graphic,,,Question Mark|Fragezeichen
64,40,@,graphic,,,Commercial At|At-Zeichen
65,41,A,graphic,,,Capital letter A|Großbuchstabe A
66,42,B,graphic,,,Capital letter B|Großbuchstabe B
67,43,C,graphic,,,Capital letter C|Großbuchstabe C
68,44,D,graphic,,,Capital letter D|Großbuchstabe D
69,45,E,graphic,,,Capital letter E|Großbuchstabe E
70,46,F,graphic,,,Capital letter F|Großbuchstabe F
71,47,G,graphic,,,Capital letter G|Großbuchstabe G
72,48,H,graphic,,,Capital letter H|Großbuchstabe H
73,49,I,graphic,,,Capital letter I|Großbuchstabe I
74,4A,J,graphic,,,Capital letter J|Großbuchstabe J
75,4B,K,graphic,,,Capital letter K|Großbuchstabe K
76,4C,L,graphic,,,Capital letter L|Großbuchstabe L
77,4D,M,graphic,,,Capital letter M|Großbuchstabe M
78,4E,N,graphic,,,Capital letter N|Großbuchstabe N
79,4F,O,graphic,,,Capital letter O|Großbuchstabe O
80,50,P,graphic,,,Capital letter P|Großbuchstabe P
81,51,Q,graphic,,,Capital letter Q|Großbuchstabe Q
82,52,R,graphic,,,Capital letter R|Großbuchstabe R
83,53,S,graphic,,,Capital letter S|Großbuchstabe S
84,54,T,graphic,,,Capital letter T|Großbuchstabe T
85,55,U,graphic,,,Capital letter U|Großbuchstabe U
86,56,V,graphic,,,Capital letter V|Großbuchstabe V
87,57,W,graphic,,,Capital letter W|Großbuchstabe W
88,58,X,graphic,,,Capital letter X|Großbuchstabe X
89,59,Y,graphic,,,Capital letter Y|Großbuchstabe Y
90,5A,Z,graphic,,,Capital letter Z|Großbuchstabe Z
91,5B,[,graphic,,,Opening Bracket|eckige Klammer auf
92,5C,\,graphic,,,Reverse Slant (backslash)|umgekehrter Schrägstrich (Backslash)
93,5D,],graphic,,,Closing Bracket|eckige Klammer zu
94,5E,^,graphic,,,Circumflex|Zirkumflex
95,5F,_,graphic,,,Underline (underscore)|Unterstrich
96,60,`,graphic,,,Grave Accent|Gravis (Accent grave)
97,61,a,graphic,,,Small letter a|Kleinbuchstabe a
98,62,b,graphic,,,Small letter b|Kleinbuchstabe b
99,63,c,graphic,,,Small letter c|Kleinbuchstabe c
100,64,d,graphic,,,Small letter d|Kleinbuchstabe d
101,65,e,graphic,,,Small letter e|Kleinbuchstabe e
102,66,f,graphic,,,Small letter f|Kleinbuchstabe f
103,67,g,graphic,,,Small letter g|Kleinbuchstabe g
104,68,h,graphic,,,Small letter h|Kleinbuchstabe h
105,69,i,graphic,,,Small letter i|Kleinbuchstabe i
106,6A,j,graphic,,,Small letter j|Kleinbuchstabe j
107,6B,k,graphic,,,Small letter k|Kleinbuchstabe k
108,6C,l,graphic,,,Small letter l|Kleinbuchstabe l
109,6D,m,graphic,,,Small letter m|Kleinbuchstabe m
110,6E,n,graphic,,,Small letter n|Kleinbuchstabe n
111,6F,o,graphic,,,Small letter o|Kleinbuchstabe o
112,70,p,graphic,,,Small letter p|Kleinbuchstabe p
113,71,q,graphic,,,Small letter q|Kleinbuchstabe q
114,72,r,graphic,,,Small letter r|Kleinbuchstabe r
115,73,s,graphic,,,Small letter s|Kleinbuchstabe s
116,74,t,graphic,,,Small letter t|Kleinbuchstabe t
117,75,u,graphic,,,Small letter u|Kleinbuchstabe u
118,76,v,graphic,,,Small letter v|Kleinbuchstabe v
119,77,w,graphic,,,Small letter w|Kleinbuchstabe w
120,78,x,graphic,,,Small letter x|Kleinbuchstabe x
121,79,y,graphic,,,Small letter y|Kleinbuchstabe y
122,7A,z,graphic,,,Small letter z|Kleinbuchstabe z
123,7B,{,graphic,,,Opening Brace|geschweifte Klammer auf
124,7C,|,graphic,,,Vertical Line|senkrechter Strich
125,7D,},graphic,,,Closing Brace|geschweifte Klammer zu
126,7E,~,graphic,,,Overline (tilde)|Tilde
127,7F,DEL,control,,^?,Delete|Zeichen löschen
```

## Sources
1. V. Cerf, RFC 20 "ASCII format for Network Interchange" (October 16, 1969), reproducing USAS X3.4-1968, https://www.rfc-editor.org/rfc/rfc20.txt — code table (columns 0–7 × rows 0–15), control-character legend with CC/FE/IS classes (section 4.1), graphic-character names (section 4.2), definitions (section 5); accessed 2026-11-02.
2. de.wikipedia, "Steuerzeichen", https://de.wikipedia.org/wiki/Steuerzeichen — German names of the control characters 0–31 and 127; accessed 2026-11-02.

## Cross-check
- Definitional (single authoritative source, RFC 20 = USAS X3.4-1968). Code ↔ character mapping was generated programmatically (a helper script that is not part of the repository, Python `chr()`), and the control-character order matches the RFC 20 table (columns 0 and 1) row by row.
- English names are RFC 20's. Archaic 1969 names got the modern name in brackets: "Slant (slash)", "Reverse Slant (backslash)", "Underline (underscore)", "Overline (tilde)". RFC 20 notes that 2/3 "#" may be replaced by "£" in some applications and that several characters (e.g. @ [ \ ] ^ ` { | } ~) "should not be used in international interchange without determining that there is agreement between sender and recipient".
- German control-character names follow source 2 exactly; for LF and CR the common terms "Zeilenvorschub" and "Wagenrücklauf" were added in brackets. German names of printable characters (letters, digits, punctuation) are ordinary German usage written by me (no German standard read).
- `ctrl` (caret notation) is formula-derived (code + 64), not from RFC 20.

## Notes (en / de)
- en: ASCII is a 7-bit code (0–127). Codes 128–255 are not ASCII; their meaning depends on the code page (e.g. ISO 8859-1, Windows-1252) or UTF-8 multi-byte sequences.
- de: ASCII ist ein 7-Bit-Code (0–127). Die Codes 128–255 sind kein ASCII; ihre Bedeutung hängt von der Codepage (z. B. ISO 8859-1, Windows-1252) bzw. von UTF-8-Mehrbytefolgen ab.
- en: Line ends: Unix/Linux LF (0x0A), Windows CR LF (0x0D 0x0A), classic Mac OS CR (0x0D).
- de: Zeilenenden: Unix/Linux LF (0x0A), Windows CR LF (0x0D 0x0A), klassisches Mac OS CR (0x0D).
- en: Upper and lower case letters differ only in bit 5 (0x20): 'A' = 0x41, 'a' = 0x61.
- de: Groß- und Kleinbuchstaben unterscheiden sich nur in Bit 5 (0x20): 'A' = 0x41, 'a' = 0x61.

## Suggested finder / lookups
- Character ↔ dec/hex/bin (the ECalc `integer` dimension already formats dec/hex/bin/oct).
- Search by control abbreviation (e.g. "ESC" → 27 / 0x1B).
