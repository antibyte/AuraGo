"""Original MIT presentation catalog; IDs are the public agent contract."""
VERSION = '1.0.0'
EFFECT_GROUPS = {
 'weather': ['rain-light','rain-heavy','snow','wind-dust','wind-leaves','thunderstorm'],
 'sky': ['sky-clear','sky-cloudy','sky-sunset','sky-storm','sky-night','sky-space'],
 'lighting': ['day-night'],
 'fog': ['fog-distance','fog-ground','fog-zone'],
 'impact': ['blood-spray','blood-pool','blood-decal','metal-sparks','stone-debris','hit-flash'],
 'particles': ['fire','smoke','embers','explosion','muzzle-flash','engine-trail','magic','teleport','pickup-glow'],
 'water': ['water-lake','water-river','water-ocean','water-splash','water-ripple','underwater'],
 'shader': ['bloom','color-grade','vignette','film-grain','heat-haze','hologram','dissolve'],
}
ENVIRONMENTS = {
 'forest-day': (['sky-clear','fog-distance','day-night','color-grade'], ['forest-day','forest-night','wind-gentle']),
 'forest-night': (['sky-night','fog-ground','vignette'], ['forest-night','wind-gentle']),
 'forest-rain': (['sky-cloudy','rain-heavy','fog-distance','fog-ground'], ['rain-heavy','wind-gentle']),
 'desert': (['sky-clear','wind-dust','heat-haze','color-grade'], ['wind-strong']),
 'storm': (['sky-storm','thunderstorm','rain-heavy','fog-distance'], ['rain-heavy','wind-strong','thunder']),
 'coast': (['sky-clear','water-ocean','color-grade'], ['ocean','wind-gentle']),
 'industrial': (['sky-sunset','fog-zone','bloom'], ['industrial']),
 'space': (['sky-space','bloom','vignette'], ['space-hum']),
}
SOUND_GROUPS = {
 'ambience': ['forest-day','forest-night','rain-light','rain-heavy','wind-gentle','wind-strong','stream','ocean','industrial','space-hum'],
 'movement': ['step-grass','step-gravel','step-stone','step-wood','step-metal','step-water','jump','land'],
 'combat': ['pistol','rifle','shotgun','laser','reload','impact-metal','impact-stone','impact-flesh','explosion-small','explosion-large'],
 'world': ['water-splash','fire','thunder','door-wood','door-metal','crate-break','engine','electric-spark'],
 'feedback': ['ui-click','pickup','victory','defeat'],
}
LOOPS = set(SOUND_GROUPS['ambience'] + ['fire','engine'])
EVENTS = ['step','jump','land','shot','reload','hit','pickup','win','lose','splash','interact','engine','ui']

def effect_assets():
    out = []
    for category, ids in EFFECT_GROUPS.items():
        for id in ids:
            defaults = dict(intensity=1, scale=1, color='#9f162a' if id.startswith('blood') else '#ffffff', lifetime=30 if id.startswith('blood') else 2)
            if category == 'weather': defaults = dict(intensity=1,scale=1,color='#ffffff')
            if category == 'sky': defaults = {}
            if id == 'day-night': defaults = dict(cycle=480,hour=10,fixed=False)
            if id in ['water-lake','water-river','water-ocean']: defaults = dict(width=80,depth=80,y=-0.35,speed=0.6)
            if id == 'water-ripple': defaults = dict(scale=1,lifetime=1.4)
            if id == 'fog-distance': defaults = dict(density=0.018)
            if id in ['fog-ground','fog-zone']: defaults = dict(intensity=1,scale=1,color='#afc6d0')
            if id == 'fog-zone': defaults['radius']=12
            if category == 'shader' or id == 'underwater': defaults = dict(intensity=1)
            if id in ['hologram','dissolve']: defaults = dict(lifetime=2)
            out.append(dict(id=id, name=id.replace('-', ' ').title(), category=category, description=f'{id.replace("-", " ")} for side/top 2D and metric 3D scenes', tags=id.split('-')+[category], dimensions=['2d','3d'], defaults=defaults, files=[]))
    for id, (effects, sounds) in ENVIRONMENTS.items():
        out.append(dict(id=id,name=id.replace('-',' ').title(),category='environment',description='Coordinated atmosphere: '+', '.join(effects),tags=[id,'atmosphere','environment'],dimensions=['2d','3d'],effects=effects,sounds=sounds,defaults={},files=[]))
    return out
