"""Deterministic Studio labels for the sixteen desktop locales."""
from pathlib import Path
import json
ROOT=Path(__file__).resolve().parents[2]
LABELS={
 'cs':('Směr','Výšková úroveň','Piráti · 3D','Piráti · pohled shora','Piráti · boční pohled','Izometrické světy'),
 'da':('Retning','Højdeniveau','Pirater · 3D','Pirater · set ovenfra','Pirater · fra siden','Isometriske verdener'),
 'de':('Blickrichtung','Höhenstufe','Piraten · 3D','Piraten · Draufsicht','Piraten · Seitenansicht','Isometrische Welten'),
 'el':('Κατεύθυνση','Επίπεδο ύψους','Πειρατές · 3D','Πειρατές · κάτοψη','Πειρατές · πλάγια όψη','Ισομετρικοί κόσμοι'),
 'en':('Direction','Elevation level','Pirates · 3D','Pirates · top view','Pirates · side view','Isometric worlds'),
 'es':('Dirección','Nivel de altura','Piratas · 3D','Piratas · vista superior','Piratas · vista lateral','Mundos isométricos'),
 'fr':('Direction','Niveau de hauteur','Pirates · 3D','Pirates · vue de dessus','Pirates · vue de côté','Mondes isométriques'),
 'hi':('दिशा','ऊँचाई स्तर','समुद्री डाकू · 3D','समुद्री डाकू · ऊपरी दृश्य','समुद्री डाकू · पार्श्व दृश्य','सममितीय दुनिया'),
 'it':('Direzione','Livello di altezza','Pirati · 3D','Pirati · vista dall’alto','Pirati · vista laterale','Mondi isometrici'),
 'ja':('方向','高さレベル','海賊 · 3D','海賊 · 見下ろし','海賊 · 横視点','アイソメトリックワールド'),
 'nl':('Richting','Hoogteniveau','Piraten · 3D','Piraten · bovenaanzicht','Piraten · zijaanzicht','Isometrische werelden'),
 'no':('Retning','Høydenivå','Pirater · 3D','Pirater · sett ovenfra','Pirater · fra siden','Isometriske verdener'),
 'pl':('Kierunek','Poziom wysokości','Piraci · 3D','Piraci · widok z góry','Piraci · widok z boku','Światy izometryczne'),
 'pt':('Direção','Nível de altura','Piratas · 3D','Piratas · vista superior','Piratas · vista lateral','Mundos isométricos'),
 'sv':('Riktning','Höjdnivå','Pirater · 3D','Pirater · ovanifrån','Pirater · från sidan','Isometriska världar'),
 'zh':('朝向','高度层级','海盗 · 3D','海盗 · 俯视','海盗 · 侧视','等距世界'),
}
KEYS=['assets_direction','assets_elevation','pack_aurago_pirates_3d','pack_aurago_pirates_topdown','pack_aurago_pirates_side','pack_aurago_isometric']
LAYERS={
'cs':('Střecha','Přední stěna'),'da':('Tag','Forvæg'),'de':('Dach','Vorderwand'),'el':('Στέγη','Μπροστινός τοίχος'),
'en':('Roof','Front wall'),'es':('Techo','Pared frontal'),'fr':('Toit','Mur avant'),'hi':('छत','सामने की दीवार'),
'it':('Tetto','Parete anteriore'),'ja':('屋根','前壁'),'nl':('Dak','Voorwand'),'no':('Tak','Frontvegg'),
'pl':('Dach','Przednia ściana'),'pt':('Telhado','Parede frontal'),'sv':('Tak','Främre vägg'),'zh':('屋顶','前墙')}
MISMATCH={
'cs':'Vybrané modely a obrázky musí odpovídat rozměru hry (2D/3D).',
'da':'De valgte modeller og billeder skal passe til spillets dimension (2D/3D).',
'de':'Die ausgewählten Modelle und Grafiken müssen zur Dimension des Spiels passen (2D/3D).',
'el':'Τα επιλεγμένα μοντέλα και γραφικά πρέπει να ταιριάζουν στη διάσταση του παιχνιδιού (2D/3D).',
'en':'Selected models and artwork must match the game dimension (2D/3D).',
'es':'Los modelos y gráficos seleccionados deben coincidir con la dimensión del juego (2D/3D).',
'fr':'Les modèles et graphismes sélectionnés doivent correspondre à la dimension du jeu (2D/3D).',
'hi':'चुने गए मॉडल और चित्र गेम के आयाम (2D/3D) से मेल खाने चाहिए।',
'it':'I modelli e la grafica selezionati devono corrispondere alla dimensione del gioco (2D/3D).',
'ja':'選択したモデルと画像はゲームの次元（2D/3D）に合わせてください。',
'nl':'De gekozen modellen en afbeeldingen moeten passen bij de dimensie van het spel (2D/3D).',
'no':'Valgte modeller og bilder må passe til spillets dimensjon (2D/3D).',
'pl':'Wybrane modele i grafika muszą pasować do wymiaru gry (2D/3D).',
'pt':'Os modelos e gráficos selecionados devem corresponder à dimensão do jogo (2D/3D).',
'sv':'Valda modeller och bilder måste passa spelets dimension (2D/3D).',
'zh':'所选模型和图像必须与游戏维度（2D/3D）匹配。'}
VESSELS={
'cs':('Lodě','Ponorky'),'da':('Skibe','Ubåde'),'de':('Schiffe','Unterwasserfahrzeuge'),
'el':('Πλοία','Υποβρύχια'),'en':('Ships','Submersibles'),'es':('Barcos','Sumergibles'),
'fr':('Navires','Submersibles'),'hi':('जहाज़','पनडुब्बियाँ'),'it':('Navi','Sommergibili'),
'ja':('船舶','潜水艇'),'nl':('Schepen','Duikboten'),'no':('Skip','Ubåter'),
'pl':('Statki','Pojazdy podwodne'),'pt':('Navios','Submersíveis'),'sv':('Fartyg','Ubåtar'),'zh':('船舶','潜水器')}
for locale,labels in LABELS.items():
    path=ROOT/'ui/lang/desktop'/f'{locale}.json';text=path.read_text(encoding='utf-8');data=json.loads(text)
    additions={f'game_maker.{key}':label for key,label in zip(KEYS,labels)}
    additions['game_maker.assets_dimension_mismatch']=MISMATCH[locale]
    additions.update({f'game_maker.asset_layer_{key}':value for key,value in zip(('roof','front-wall'),LAYERS[locale])})
    additions.update({f'game_maker.model_category_{key}':value for key,value in zip(('ships','submarines'),VESSELS[locale])})
    # Preserve the original ordering and formatting of the large locale files.
    missing={key:value for key,value in additions.items() if key not in data}
    if missing:path.write_text('{\n'+''.join('  '+json.dumps(k)+': '+json.dumps(v,ensure_ascii=False)+',\n' for k,v in missing.items())+text.split('\n',1)[1],encoding='utf-8')
