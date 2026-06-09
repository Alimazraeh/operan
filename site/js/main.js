// Operan Website — Main JavaScript
// Enterprise Agentic Workforce Infrastructure

/* ============================================================
   Scroll-triggered header
   ============================================================ */
(function initNavScroll() {
  const nav = document.querySelector('.nav');
  if (!nav) return;

  const onScroll = () => {
    if (window.scrollY > 50) {
      nav.classList.add('nav--scrolled');
    } else {
      nav.classList.remove('nav--scrolled');
    }
  };

  window.addEventListener('scroll', onScroll, { passive: true });
  onScroll();
})();

/* ============================================================
   Mobile nav toggle
   ============================================================ */
(function initMobileNav() {
  const toggle = document.querySelector('.nav__toggle');
  const links = document.querySelector('.nav__links');
  if (!toggle || !links) return;

  toggle.addEventListener('click', () => {
    toggle.classList.toggle('nav__toggle--active');
    links.classList.toggle('nav__links--open');
  });

  // Close on link click
  links.querySelectorAll('.nav__link').forEach(link => {
    link.addEventListener('click', () => {
      toggle.classList.remove('nav__toggle--active');
      links.classList.remove('nav__links--open');
    });
  });
})();

/* ============================================================
   Scroll reveal
   ============================================================ */
(function initScrollReveal() {
  const reveals = document.querySelectorAll('.reveal');
  if (!reveals.length) return;

  const observer = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
      if (entry.isIntersecting) {
        entry.target.classList.add('reveal--visible');
        observer.unobserve(entry.target);
      }
    });
  }, {
    threshold: 0.1,
    rootMargin: '0px 0px -60px 0px'
  });

  reveals.forEach(el => observer.observe(el));
})();

/* ============================================================
   Counter animation
   ============================================================ */
(function initCounters() {
  const counters = document.querySelectorAll('.stat__value');
  if (!counters.length) return;

  const animate = (el) => {
    const target = parseInt(el.dataset.target, 10);
    const suffix = el.textContent.replace(/[0-9]/g, '');
    const duration = 2000;
    const start = performance.now();

    const step = (now) => {
      const progress = Math.min((now - start) / duration, 1);
      // Ease out cubic
      const eased = 1 - Math.pow(1 - progress, 3);
      const current = Math.round(eased * target);
      el.textContent = current.toLocaleString() + suffix;
      if (progress < 1) {
        requestAnimationFrame(step);
      }
    };

    requestAnimationFrame(step);
  };

  const observer = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
      if (entry.isIntersecting) {
        animate(entry.target);
        observer.unobserve(entry.target);
      }
    });
  }, { threshold: 0.5 });

  counters.forEach(el => observer.observe(el));
})();

/* ============================================================
   Tab navigation
   ============================================================ */
(function initTabs() {
  const tabs = document.querySelectorAll('.tab-btn');
  const panels = document.querySelectorAll('.tab-panel');
  if (!tabs.length || !panels.length) return;

  tabs.forEach(tab => {
    tab.addEventListener('click', () => {
      const target = tab.dataset.tab;

      // Deactivate all
      tabs.forEach(t => t.classList.remove('tab-btn--active'));
      panels.forEach(p => p.classList.remove('tab-panel--active'));

      // Activate selected
      tab.classList.add('tab-btn--active');
      const panel = document.querySelector(`.tab-panel[data-panel="${target}"]`);
      if (panel) panel.classList.add('tab-panel--active');
    });
  });
})();

/* ============================================================
   Form handling
   ============================================================ */
(function initForms() {
  const form = document.querySelector('.briefing-form');
  if (!form) return;

  form.addEventListener('submit', (e) => {
    e.preventDefault();

    const btn = form.querySelector('button[type="submit"]');
    const originalText = btn.innerHTML;

    btn.innerHTML = '<span>Sending...</span>';
    btn.disabled = true;

    // Simulate submission
    setTimeout(() => {
      btn.innerHTML = '<span>Request Sent ✓</span>';
      btn.style.background = 'linear-gradient(135deg, #10B981 0%, #059669 100%)';

      setTimeout(() => {
        btn.innerHTML = originalText;
        btn.style.background = '';
        btn.disabled = false;
        form.reset();
      }, 3000);
    }, 1500);
  });
})();

/* ============================================================
   Three.js Hero Scene — Operan Digital Workforce
   Impressive 3D visualization: glowing department nodes,
   orbital networks, data streams, and particle fields
   ============================================================ */
(function initHeroScene() {
  const canvas = document.querySelector('.hero__canvas');
  if (!canvas) return;

  if (typeof THREE === 'undefined') {
    console.warn('Three.js not loaded — hero canvas disabled');
    return;
  }

  const scene = new THREE.Scene();
  const camera = new THREE.PerspectiveCamera(50, window.innerWidth / window.innerHeight, 0.1, 1000);
  camera.position.set(0, 0, 28);

  const renderer = new THREE.WebGLRenderer({
    canvas: canvas,
    antialias: true,
    alpha: true
  });
  renderer.setSize(window.innerWidth, window.innerHeight);
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));

  // ── Central Core (glowing department nexus) ──────────────
  const coreGroup = new THREE.Group();

  // Inner core — bright glowing sphere
  const coreGeo = new THREE.IcosahedronGeometry(4.4, 2);
  const coreMat = new THREE.MeshBasicMaterial({
    color: 0x00D4FF,
    wireframe: true,
    transparent: true,
    opacity: 0.35
  });
  const core = new THREE.Mesh(coreGeo, coreMat);
  coreGroup.add(core);

  // Outer core shell
  const outerCoreGeo = new THREE.IcosahedronGeometry(6.4, 1);
  const outerCoreMat = new THREE.MeshBasicMaterial({
    color: 0x3B82F6,
    wireframe: true,
    transparent: true,
    opacity: 0.15
  });
  const outerCore = new THREE.Mesh(outerCoreGeo, outerCoreMat);
  coreGroup.add(outerCore);

  // Energy shell
  const energyGeo = new THREE.IcosahedronGeometry(9.0, 1);
  const energyMat = new THREE.MeshBasicMaterial({
    color: 0x00D4FF,
    wireframe: true,
    transparent: true,
    opacity: 0.06
  });
  const energyShell = new THREE.Mesh(energyGeo, energyMat);
  coreGroup.add(energyShell);

  coreGroup.position.set(4, 0, -2);
  scene.add(coreGroup);

  // ── Department Orbital Rings + Nodes ─────────────────────
  const departments = [
    { name: 'Human Resources', color: 0x00D4FF },
    { name: 'Finance', color: 0x3B82F6 },
    { name: 'Legal', color: 0x8B5CF6 },
    { name: 'Engineering', color: 0x10B981 },
    { name: 'Research', color: 0xF59E0B },
    { name: 'Compliance', color: 0xEF4444 }
  ];

  const deptNodes = [];
  const deptOrbits = [];

  departments.forEach((dept, i) => {
    const radius = 14 + i * 1.6;
    const tilt = 0.3 + (i % 3) * 0.2;

    // Orbital ring
    const orbitGeo = new THREE.TorusGeometry(radius, 0.008, 4, 128);
    const orbitMat = new THREE.MeshBasicMaterial({
      color: dept.color,
      transparent: true,
      opacity: 0.12
    });
    const orbit = new THREE.Mesh(orbitGeo, orbitMat);
    orbit.position.copy(coreGroup.position);
    orbit.rotation.x = Math.PI / 2 + tilt;
    orbit.rotation.z = (i / departments.length) * Math.PI;
    scene.add(orbit);
    deptOrbits.push(orbit);

    // Glowing node on orbit
    const nodeGeo = new THREE.OctahedronGeometry(0.24, 0);
    const nodeMat = new THREE.MeshBasicMaterial({
      color: dept.color,
      transparent: true,
      opacity: 0.9
    });
    const node = new THREE.Mesh(nodeGeo, nodeMat);
    const angle = (i / departments.length) * Math.PI * 2;
    node.position.set(
      coreGroup.position.x + Math.cos(angle) * radius,
      Math.sin(angle) * radius * 0.6,
      coreGroup.position.z + Math.sin(angle) * radius
    );
    node.userData = {
      orbitRadius: radius,
      angle: angle,
      speed: 0.15 + i * 0.03,
      tilt: tilt,
      center: coreGroup.position.clone(),
      tiltAngle: (i / departments.length) * Math.PI,
      orbitIndex: i
    };
    scene.add(node);
    deptNodes.push(node);

    // Node glow ring
    const ringGeo = new THREE.RingGeometry(0.25, 0.35, 32);
    const ringMat = new THREE.MeshBasicMaterial({
      color: dept.color,
      transparent: true,
      opacity: 0.15,
      side: THREE.DoubleSide
    });
    const ring = new THREE.Mesh(ringGeo, ringMat);
    ring.position.copy(node.position);
    ring.lookAt(camera.position);
    node.userData.ring = ring;
    scene.add(ring);
  });

  // ── Network Connections (data streams) ───────────────────
  const connectionGroup = new THREE.Group();
  const lineMaterial = new THREE.LineBasicMaterial({
    color: 0x00D4FF,
    transparent: true,
    opacity: 0.04
  });

  // Connect department nodes to core
  deptNodes.forEach((node, i) => {
    const points = [coreGroup.position.clone(), node.position.clone()];
    const lineGeo = new THREE.BufferGeometry().setFromPoints(points);
    const line = new THREE.Line(lineGeo, lineMaterial.clone());
    line.userData = { nodeIndex: i };
    connectionGroup.add(line);
  });

  // Connect adjacent department nodes
  for (let i = 0; i < deptNodes.length; i++) {
    const next = (i + 1) % deptNodes.length;
    const points = [deptNodes[i].position.clone(), deptNodes[next].position.clone()];
    const lineGeo = new THREE.BufferGeometry().setFromPoints(points);
    const line = new THREE.Line(lineGeo, new THREE.LineBasicMaterial({
      color: 0x3B82F6,
      transparent: true,
      opacity: 0.03
    }));
    connectionGroup.add(line);
  }

  scene.add(connectionGroup);

  // ── Data Flow Particles (moving along connections) ───────
  const dataParticles = [];
  const dataParticleMat = new THREE.PointsMaterial({
    color: 0x00D4FF,
    size: 0.12,
    transparent: true,
    opacity: 0.8
  });

  deptNodes.forEach((node, i) => {
    const geo = new THREE.BufferGeometry();
    const positions = new Float32Array(3);
    positions[0] = coreGroup.position.x;
    positions[1] = coreGroup.position.y;
    positions[2] = coreGroup.position.z;
    geo.setAttribute('position', new THREE.BufferAttribute(positions, 3));
    const particle = new THREE.Points(geo, dataParticleMat.clone());
    particle.userData = {
      target: node.position.clone(),
      progress: Math.random(),
      speed: 0.3 + Math.random() * 0.4
    };
    scene.add(particle);
    dataParticles.push(particle);
  });

  // ── Floating Network Nodes ───────────────────────────────
  const nodeCount = 80;
  const networkNodes = [];
  const networkNodeMat = new THREE.MeshBasicMaterial({
    color: 0x00D4FF,
    transparent: true,
    opacity: 0.5
  });

  for (let i = 0; i < nodeCount; i++) {
    const phi = Math.acos(-1 + (2 * i) / nodeCount);
    const theta = Math.sqrt(nodeCount * Math.PI) * phi;
    const radius = 8 + Math.random() * 6;

    const geo = new THREE.SphereGeometry(0.04 + Math.random() * 0.05, 6, 6);
    const node = new THREE.Mesh(geo, networkNodeMat.clone());
    node.position.set(
      coreGroup.position.x + radius * Math.cos(theta) * Math.sin(phi),
      radius * Math.sin(theta) * Math.sin(phi),
      coreGroup.position.z + radius * Math.cos(phi)
    );
    node.userData = {
      baseOpacity: 0.2 + Math.random() * 0.4,
      phase: Math.random() * Math.PI * 2,
      speed: 0.3 + Math.random() * 0.6
    };
    scene.add(node);
    networkNodes.push(node);
  }

  // ── Connection Lines (network mesh) ──────────────────────
  const meshMat = new THREE.LineBasicMaterial({
    color: 0x00D4FF,
    transparent: true,
    opacity: 0.025
  });

  for (let i = 0; i < 100; i++) {
    const idxA = Math.floor(Math.random() * nodeCount);
    const idxB = Math.floor(Math.random() * nodeCount);
    if (idxA === idxB) continue;
    const points = [networkNodes[idxA].position, networkNodes[idxB].position];
    const lineGeo = new THREE.BufferGeometry().setFromPoints(points);
    const line = new THREE.Line(lineGeo, meshMat.clone());
    scene.add(line);
  }

  // ── Ambient Particle Field ───────────────────────────────
  const particleCount = 300;
  const particleGeo = new THREE.BufferGeometry();
  const positions = new Float32Array(particleCount * 3);

  for (let i = 0; i < particleCount; i++) {
    positions[i * 3] = (Math.random() - 0.5) * 80;
    positions[i * 3 + 1] = (Math.random() - 0.5) * 50;
    positions[i * 3 + 2] = (Math.random() - 0.5) * 40 - 5;
  }

  particleGeo.setAttribute('position', new THREE.BufferAttribute(positions, 3));
  const particleMat = new THREE.PointsMaterial({
    color: 0x00D4FF,
    size: 0.04,
    transparent: true,
    opacity: 0.25
  });
  const particles = new THREE.Points(particleGeo, particleMat);
  scene.add(particles);

  // ── Mouse tracking ───────────────────────────────────────
  let mouseX = 0;
  let mouseY = 0;

  document.addEventListener('mousemove', (e) => {
    mouseX = (e.clientX / window.innerWidth) * 2 - 1;
    mouseY = -(e.clientY / window.innerHeight) * 2 + 1;
  });

  // ── Animation loop ───────────────────────────────────────
  const clock = new THREE.Clock();

  function animate() {
    requestAnimationFrame(animate);
    const elapsed = clock.getElapsedTime();

    // Core rotations
    core.rotation.y = elapsed * 0.3;
    core.rotation.x = elapsed * 0.15;
    outerCore.rotation.y = -elapsed * 0.2;
    outerCore.rotation.z = elapsed * 0.1;
    energyShell.rotation.y = elapsed * 0.1;
    energyShell.rotation.x = elapsed * 0.05;

    // Pulsing energy shell
    const pulse = 1 + 0.15 * Math.sin(elapsed * 2);
    energyShell.scale.set(pulse, pulse, pulse);

    // Department nodes orbiting
    deptNodes.forEach((node) => {
      const d = node.userData;
      d.angle += d.speed * 0.008;

      const x = d.center.x + Math.cos(d.angle) * d.orbitRadius;
      const y = Math.sin(d.angle) * d.orbitRadius * 0.6;
      const z = d.center.z + Math.sin(d.angle) * d.orbitRadius;

      node.position.set(x, y, z);

      // Ring faces camera
      if (d.ring) {
        d.ring.position.copy(node.position);
        d.ring.lookAt(camera.position);
      }
    });

    // Data flow particles
    dataParticles.forEach((p) => {
      p.userData.progress += p.userData.speed * 0.005;
      if (p.userData.progress > 1) p.userData.progress = 0;
      const t = p.userData.progress;
      const pos = p.geometry.attributes.position;
      pos.array[0] = coreGroup.position.x + (p.userData.target.x - coreGroup.position.x) * t;
      pos.array[1] = coreGroup.position.y + (p.userData.target.y - coreGroup.position.y) * t;
      pos.array[2] = coreGroup.position.z + (p.userData.target.z - coreGroup.position.z) * t;
      pos.needsUpdate = true;
    });

    // Network nodes pulse
    networkNodes.forEach((node) => {
      if (node.userData.phase !== undefined) {
        node.material.opacity = node.userData.baseOpacity * (0.5 + 0.5 * Math.sin(elapsed * node.userData.speed + node.userData.phase));
      }
    });

    // Slow rotation
    particles.rotation.y = elapsed * 0.005;
    connectionGroup.rotation.y = elapsed * 0.02;

    // Mouse parallax
    const targetX = mouseX * 2;
    const targetY = mouseY * 1.5;
    camera.position.x += (targetX - camera.position.x) * 0.03;
    camera.position.y += (targetY - camera.position.y) * 0.03;
    camera.lookAt(coreGroup.position);

    renderer.render(scene, camera);
  }

  animate();

  // ── Resize ───────────────────────────────────────────────
  window.addEventListener('resize', () => {
    camera.aspect = window.innerWidth / window.innerHeight;
    camera.updateProjectionMatrix();
    renderer.setSize(window.innerWidth, window.innerHeight);
  });
})();
