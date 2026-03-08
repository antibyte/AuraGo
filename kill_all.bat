@echo off
echo Beende AuraGo Supervisor und Lifeboat...
taskkill /F /IM aurago.exe /T
taskkill /F /IM lifeboat.exe /T
echo Alles beendet.
pause
