@echo off
"%~dp0zig\zig.exe" cc -target x86_64-windows-msvc %*
