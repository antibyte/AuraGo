// AuraGo – login page logic
// Extracted from login.html

        // ═══════════════════════════════════════════════════════════════
        // EPIC WEBGL BACKGROUND - Neural Network Particle System
        // ═══════════════════════════════════════════════════════════════
        
        const canvas = document.getElementById('bg-canvas');
        const cssFallback = document.getElementById('css-bg');
        
        let scene, camera, renderer, particles, lines;
        let mouseX = 0, mouseY = 0;
        let time = 0;

        function showCSSFallback() {
            canvas.classList.add('is-hidden');
            cssFallback.classList.remove('is-hidden');
        }

        function showWebGL() {
            canvas.classList.remove('is-hidden');
            cssFallback.classList.add('is-hidden');
        }
        
        // Check WebGL support
        function webglAvailable() {
            try {
                const testCanvas = document.createElement('canvas');
                return !!(window.WebGLRenderingContext && 
                    (testCanvas.getContext('webgl') || testCanvas.getContext('experimental-webgl')));
            } catch(e) {
                return false;
            }
        }
        
        // Initialize after Three.js is loaded
        function initBackground() {
            if (webglAvailable() && typeof THREE !== 'undefined' && !window._threeLoadFailed) {
                try {
                    initWebGL();
                } catch(e) {
                    console.warn('[AuraGo] WebGL init failed, using CSS fallback:', e);
                    showCSSFallback();
                }
            } else {
                // Show CSS fallback (Three.js unavailable or WebGL not supported)
                showCSSFallback();
            }
        }
        
        // Wait for Three.js to load
        if (document.readyState === 'complete') {
            initBackground();
        } else {
            window.addEventListener('load', initBackground);
        }

        // ── i18n: populate text ──
        (function applyI18N() {
            const el = id => document.getElementById(id);
            el('loginSubtitle').textContent       = t('login.subtitle');
            el('lblPassword').textContent          = t('login.password_label');
            el('eyeBtn').title                     = t('login.password_show');
            el('totpDivider').textContent           = t('login.totp_divider');
            el('lblTotp').textContent               = t('login.totp_label');
            el('btnText').textContent               = t('login.btn_submit');
            if (TOTP_ENABLED) el('totpSection').classList.remove('is-hidden');
        })();
        
        function initWebGL() {
            // Scene setup
            scene = new THREE.Scene();
            scene.fog = new THREE.FogExp2(0x0b0f1a, 0.001);
            
            // Camera
            camera = new THREE.PerspectiveCamera(75, window.innerWidth / window.innerHeight, 0.1, 1000);
            camera.position.z = 50;
            
            // Renderer
            renderer = new THREE.WebGLRenderer({ 
                canvas: canvas, 
                alpha: true, 
                antialias: true 
            });
            renderer.setSize(window.innerWidth, window.innerHeight);
            renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
            renderer.setClearColor(0x0b0f1a, 1);
            
            // Create particle system
            createParticles();
            createConnections();
            createFloatingOrbs();

            // WebGL ready — show canvas, hide CSS fallback
            showWebGL();
            
            // Mouse tracking
            document.addEventListener('mousemove', onMouseMove, false);
            window.addEventListener('resize', onWindowResize, false);
            
            // Start animation
            animate();
        }
        
        function createParticles() {
            const particleCount = 150;
            const geometry = new THREE.BufferGeometry();
            const positions = new Float32Array(particleCount * 3);
            const colors = new Float32Array(particleCount * 3);
            const sizes = new Float32Array(particleCount);
            
            const color1 = new THREE.Color(0x2dd4bf); // Teal
            const color2 = new THREE.Color(0x0d9488); // Darker teal
            
            for (let i = 0; i < particleCount; i++) {
                // Position
                positions[i * 3] = (Math.random() - 0.5) * 100;
                positions[i * 3 + 1] = (Math.random() - 0.5) * 100;
                positions[i * 3 + 2] = (Math.random() - 0.5) * 50;
                
                // Color gradient
                const mixRatio = Math.random();
                const color = color1.clone().lerp(color2, mixRatio);
                colors[i * 3] = color.r;
                colors[i * 3 + 1] = color.g;
                colors[i * 3 + 2] = color.b;
                
                // Size
                sizes[i] = Math.random() * 2 + 0.5;
            }
            
            geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
            geometry.setAttribute('color', new THREE.BufferAttribute(colors, 3));
            geometry.setAttribute('size', new THREE.BufferAttribute(sizes, 1));
            
            // Custom shader material for glowing particles
            const material = new THREE.ShaderMaterial({
                uniforms: {
                    time: { value: 0 }
                },
                vertexShader: `
                    attribute float size;
                    attribute vec3 color;
                    varying vec3 vColor;
                    uniform float time;
                    
                    void main() {
                        vColor = color;
                        vec3 pos = position;
                        
                        // Gentle floating animation
                        pos.y += sin(time * 0.5 + position.x * 0.1) * 2.0;
                        pos.x += cos(time * 0.3 + position.y * 0.1) * 1.0;
                        
                        vec4 mvPosition = modelViewMatrix * vec4(pos, 1.0);
                        gl_PointSize = size * (300.0 / -mvPosition.z);
                        gl_Position = projectionMatrix * mvPosition;
                    }
                `,
                fragmentShader: `
                    varying vec3 vColor;
                    
                    void main() {
                        // Create circular glow
                        vec2 center = gl_PointCoord - vec2(0.5);
                        float dist = length(center);
                        float alpha = 1.0 - smoothstep(0.0, 0.5, dist);
                        
                        // Core brightness
                        float core = 1.0 - smoothstep(0.0, 0.2, dist);
                        
                        gl_FragColor = vec4(vColor * (0.8 + core * 0.4), alpha * 0.8);
                    }
                `,
                transparent: true,
                blending: THREE.AdditiveBlending,
                depthWrite: false
            });
            
            particles = new THREE.Points(geometry, material);
            scene.add(particles);
        }
        
        function createConnections() {
            // Create lines between nearby particles
            const lineMaterial = new THREE.LineBasicMaterial({
                color: 0x2dd4bf,
                transparent: true,
                opacity: 0.15,
                blending: THREE.AdditiveBlending
            });
            
            const lineGeometry = new THREE.BufferGeometry();
            const linePositions = new Float32Array(150 * 150 * 6); // Max possible lines
            lineGeometry.setAttribute('position', new THREE.BufferAttribute(linePositions, 3));
            lineGeometry.setDrawRange(0, 0);
            
            lines = new THREE.LineSegments(lineGeometry, lineMaterial);
            lines.userData = { positions: linePositions };
            scene.add(lines);
        }
        
        function createFloatingOrbs() {
            // Large glowing orbs in background
            const orbGeometry = new THREE.SphereGeometry(1, 32, 32);
            
            const orbPositions = [
                { x: -30, y: 20, z: -20, scale: 8, color: 0x2dd4bf },
                { x: 35, y: -15, z: -30, scale: 6, color: 0x0d9488 },
                { x: 0, y: -25, z: -10, scale: 10, color: 0x115e59 }
            ];
            
            orbPositions.forEach((orb, i) => {
                const material = new THREE.MeshBasicMaterial({
                    color: orb.color,
                    transparent: true,
                    opacity: 0.08,
                    blending: THREE.AdditiveBlending
                });
                
                const mesh = new THREE.Mesh(orbGeometry, material);
                mesh.position.set(orb.x, orb.y, orb.z);
                mesh.scale.setScalar(orb.scale);
                mesh.userData = { 
                    originalPos: { ...orb },
                    phase: i * Math.PI / 3 
                };
                
                scene.add(mesh);
                
                // Add glow halo
                const haloMaterial = new THREE.MeshBasicMaterial({
                    color: orb.color,
                    transparent: true,
                    opacity: 0.03,
                    blending: THREE.AdditiveBlending
                });
                const halo = new THREE.Mesh(orbGeometry, haloMaterial);
                halo.position.copy(mesh.position);
                halo.scale.setScalar(orb.scale * 1.5);
                halo.userData = mesh.userData;
                scene.add(halo);
            });
        }
        
        function updateConnections() {
            const positions = particles.geometry.attributes.position.array;
            const linePositions = lines.userData.positions;
            let lineIndex = 0;
            const maxDistance = 20;
            
            for (let i = 0; i < 150; i++) {
                let connections = 0;
                for (let j = i + 1; j < 150 && connections < 3; j++) {
                    const dx = positions[i * 3] - positions[j * 3];
                    const dy = positions[i * 3 + 1] - positions[j * 3 + 1];
                    const dz = positions[i * 3 + 2] - positions[j * 3 + 2];
                    const dist = Math.sqrt(dx * dx + dy * dy + dz * dz);
                    
                    if (dist < maxDistance) {
                        const alpha = 1 - (dist / maxDistance);
                        
                        linePositions[lineIndex++] = positions[i * 3];
                        linePositions[lineIndex++] = positions[i * 3 + 1];
                        linePositions[lineIndex++] = positions[i * 3 + 2];
                        linePositions[lineIndex++] = positions[j * 3];
                        linePositions[lineIndex++] = positions[j * 3 + 1];
                        linePositions[lineIndex++] = positions[j * 3 + 2];
                        
                        connections++;
                    }
                }
            }
            
            lines.geometry.attributes.position.needsUpdate = true;
            lines.geometry.setDrawRange(0, lineIndex / 3);
        }
        
        function onMouseMove(event) {
            mouseX = (event.clientX / window.innerWidth) * 2 - 1;
            mouseY = -(event.clientY / window.innerHeight) * 2 + 1;
        }
        
        function onWindowResize() {
            camera.aspect = window.innerWidth / window.innerHeight;
            camera.updateProjectionMatrix();
            renderer.setSize(window.innerWidth, window.innerHeight);
        }
        
        function animate() {
            requestAnimationFrame(animate);
            
            time += 0.01;
            
            if (particles) {
                // Update shader time
                particles.material.uniforms.time.value = time;
                
                // Rotate entire system slowly
                particles.rotation.y = time * 0.05;
                particles.rotation.x = Math.sin(time * 0.02) * 0.1;
                
                // Update positions for connection lines
                const positions = particles.geometry.attributes.position.array;
                for (let i = 0; i < 150; i++) {
                    const ix = i * 3;
                    const iy = i * 3 + 1;
                    const iz = i * 3 + 2;
                    
                    // Add subtle movement
                    positions[iy] += Math.sin(time + positions[ix] * 0.1) * 0.02;
                }
                particles.geometry.attributes.position.needsUpdate = true;
                
                // Update connections
                updateConnections();
                lines.rotation.y = time * 0.05;
                lines.rotation.x = Math.sin(time * 0.02) * 0.1;
            }
            
            // Animate orbs
            scene.children.forEach(child => {
                if (child.userData && child.userData.originalPos) {
                    const { originalPos, phase } = child.userData;
                    child.position.x = originalPos.x + Math.sin(time * 0.5 + phase) * 5;
                    child.position.y = originalPos.y + Math.cos(time * 0.3 + phase) * 3;
                }
            });
            
            // Camera follows mouse subtly
            camera.position.x += (mouseX * 2 - camera.position.x) * 0.02;
            camera.position.y += (mouseY * 2 - camera.position.y) * 0.02;
            camera.lookAt(scene.position);
            
            renderer.render(scene, camera);
        }
        
        // ═══════════════════════════════════════════════════════════════
        // LOGIN FUNCTIONALITY
        // ═══════════════════════════════════════════════════════════════
        
        if (TOTP_ENABLED) {
            document.getElementById('totpSection').style.display = '';
        }
        
        window.addEventListener('load', () => {
            document.getElementById('password').focus();
        });
        
        function toggleEye(btn, inputId) {
            const inp = document.getElementById(inputId);
            if (inp.type === 'password') {
                inp.type = 'text';
                btn.textContent = '🙈';
            } else {
                inp.type = 'password';
                btn.textContent = '👁';
            }
        }
        
        function toggleTheme() {
            const html = document.documentElement;
            const next = html.getAttribute('data-theme') === 'dark' ? 'light' : 'dark';
            html.setAttribute('data-theme', next);
            localStorage.setItem('aurago-theme', next);
            
            // Update WebGL clear color for theme
            if (renderer) {
                renderer.setClearColor(next === 'dark' ? 0x0b0f1a : 0xf8fafc, 1);
                if (particles) {
                    const colors = particles.geometry.attributes.color.array;
                    const color1 = new THREE.Color(next === 'dark' ? 0x2dd4bf : 0x0f766e);
                    const color2 = new THREE.Color(next === 'dark' ? 0x0d9488 : 0x0d9488);
                    
                    for (let i = 0; i < 150; i++) {
                        const mixRatio = Math.random();
                        const color = color1.clone().lerp(color2, mixRatio);
                        colors[i * 3] = color.r;
                        colors[i * 3 + 1] = color.g;
                        colors[i * 3 + 2] = color.b;
                    }
                    particles.geometry.attributes.color.needsUpdate = true;
                }
            }
        }
        
        function showError(msg) {
            const el = document.getElementById('loginError');
            el.textContent = msg;
            el.classList.add('visible');
        }
        
        function clearError() {
            document.getElementById('loginError').classList.remove('visible');
            document.getElementById('password').classList.remove('error-field');
            document.getElementById('totpCode').classList.remove('error-field');
        }
        
        async function submitLogin() {
            clearError();
            const password = document.getElementById('password').value;
            const totpCode = document.getElementById('totpCode').value;
            
            if (!password) {
                document.getElementById('password').classList.add('error-field');
                document.getElementById('password').focus();
                return;
            }
            
            const btn = document.getElementById('btnLogin');
            btn.disabled = true;
            document.getElementById('btnText').innerHTML = '<div class="spinner-sm"></div>';
            
            try {
                const redirect = REDIRECT_URL || '/';
                const resp = await fetch('/auth/login', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ password, totp_code: totpCode, redirect }),
                });
                
                const data = await resp.json();
                
                if (resp.ok && data.ok) {
                    window.location.href = data.redirect || '/';
                } else {
                    showError(data.error || t('login.error_failed'));
                    document.getElementById('password').classList.add('error-field');
                    if (TOTP_ENABLED && data.error && data.error.toLowerCase().includes('authenticator')) {
                        document.getElementById('totpCode').classList.add('error-field');
                        document.getElementById('totpCode').focus();
                    } else {
                        document.getElementById('password').focus();
                    }
                }
            } catch (err) {
                showError(t('login.error_network'));
            } finally {
                btn.disabled = false;
                document.getElementById('btnText').textContent = t('login.btn_submit');
            }
        }
