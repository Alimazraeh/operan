// OPERAN — Shared JavaScript
// ═══════════════════════════════════════════════════════════
// KAFKA EVENT DASHBOARD (live topic-lane stream — Canvas 2D)
// ═══════════════════════════════════════════════════════════
const OperanDataVisual = {
  canvas: null, ctx: null, running: false, raf: null,
  lanes: [], w: 0, h: 0, dpr: 1, lastT: 0, reduced: false,

  // Topics mirror the real Operan platform event bus.
  _topics: [
    { topic: 'operan.tenant.control', color: '#38BDF8', events: ['tenant.created', 'plan.updated', 'quota.synced'] },
    { topic: 'operan.iam.auth',       color: '#60A5FA', events: ['login.success', 'token.issued', 'mfa.verified'] },
    { topic: 'operan.orchestration',  color: '#22D3EE', events: ['pipeline.started', 'step.completed', 'agent.dispatched', 'pipeline.completed'] },
    { topic: 'operan.agent.registry', color: '#2DD4BF', events: ['agent.registered', 'heartbeat.ok', 'agent.scaled'] },
    { topic: 'operan.dept.templates', color: '#E2B658', events: ['department.deployed', 'template.rendered'] },
    { topic: 'operan.memory.fabric',  color: '#A78BFA', events: ['vector.upserted', 'memory.recalled', 'embed.done'] },
    { topic: 'operan.supervision',    color: '#818CF8', events: ['gate.raised', 'gate.approved', 'intervention.logged'] },
    { topic: 'operan.observability',  color: '#34D399', events: ['span.recorded', 'metric.emitted', 'alert.cleared'] }
  ],
  // Mostly healthy, with the occasional warning and rare error.
  _statusRoll: ['ok', 'ok', 'ok', 'ok', 'ok', 'ok', 'ok', 'ok', 'warn', 'ok', 'ok', 'err'],

  init: function (canvasId) {
    this.canvas = document.getElementById(canvasId);
    if (!this.canvas) return;
    this.ctx = this.canvas.getContext('2d');
    if (!this.ctx) return;
    this.reduced = !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
    this.running = true;

    var self = this;
    this._buildLanes();
    this._resize();
    window.addEventListener('resize', function () { self._resize(); });

    this._prefill();                       // populate lanes so the stream reads as live from frame 1
    if (this.reduced) { this._draw(); return; }
    this.lastT = performance.now();
    this.raf = requestAnimationFrame(function (t) { self._loop(t); });
  },

  _rand: function (arr) { return arr[Math.floor(Math.random() * arr.length)]; },

  _hexA: function (hex, a) {
    var n = parseInt(hex.slice(1), 16);
    return 'rgba(' + ((n >> 16) & 255) + ',' + ((n >> 8) & 255) + ',' + (n & 255) + ',' + a + ')';
  },

  _buildLanes: function () {
    this.lanes = this._topics.map(function (t, i) {
      return {
        topic: t.topic, color: t.color, events: t.events,
        partitions: 3,
        offset: 40100 + Math.floor(Math.random() * 8000),
        speed: 50 + (i % 4) * 5,            // px/sec — gentle per-lane variety
        gap: 4.5 + Math.random() * 2.3,     // seconds between messages
        acc: Math.random() * 2.2,
        y: 0,
        packets: []
      };
    });
  },

  _spawn: function (lane, atX) {
    lane.offset += 1;
    lane.packets.push({
      x: atX,
      event: this._rand(lane.events),
      status: this._rand(this._statusRoll),
      partition: Math.floor(Math.random() * lane.partitions),
      offset: lane.offset,
      age: 0
    });
  },

  _prefill: function () {
    // Seed lanes across the width (staggered, no spawn pulse) so the
    // dashboard looks populated the instant the page loads.
    for (var i = 0; i < this.lanes.length; i++) {
      var lane = this.lanes[i];
      for (var x = 220 + (i % 3) * 120; x < this.w + 120; x += 260) {
        this._spawn(lane, x);
        lane.packets[lane.packets.length - 1].age = 1;
      }
    }
  },

  _resize: function () {
    this.dpr = window.devicePixelRatio || 1;
    // Fall back if the viewport reports 0 (e.g. loaded in a hidden/background
    // tab) so messages always spawn off-screen right, never at the left edge.
    this.w = window.innerWidth || document.documentElement.clientWidth || 1280;
    this.h = window.innerHeight || document.documentElement.clientHeight || 800;
    this.canvas.width = this.w * this.dpr;
    this.canvas.height = this.h * this.dpr;
    this.ctx.setTransform(this.dpr, 0, 0, this.dpr, 0, 0);
    var n = this.lanes.length;
    var top = this.h * 0.09, usable = this.h * 0.82;
    for (var i = 0; i < n; i++) {
      this.lanes[i].y = Math.round(top + usable * ((i + 0.5) / n));
    }
  },

  _loop: function (t) {
    if (!this.running) return;
    var dt = (t - this.lastT) / 1000;
    if (dt > 0.05) dt = 0.05;             // clamp jumps after tab refocus
    this.lastT = t;
    this._update(dt);
    this._draw();
    var self = this;
    this.raf = requestAnimationFrame(function (tt) { self._loop(tt); });
  },

  _update: function (dt) {
    for (var i = 0; i < this.lanes.length; i++) {
      var lane = this.lanes[i];
      lane.acc += dt;
      if (lane.acc >= lane.gap) {
        lane.acc = 0;
        lane.gap = 4.5 + Math.random() * 2.3;
        this._spawn(lane, this.w + 40);
      }
      var kept = [];
      for (var j = 0; j < lane.packets.length; j++) {
        var p = lane.packets[j];
        p.x -= lane.speed * dt;
        p.age += dt;
        if (p.x > -260) kept.push(p);      // cull once fully off the left edge
      }
      lane.packets = kept;
    }
  },

  _draw: function () {
    var ctx = this.ctx, w = this.w;
    ctx.clearRect(0, 0, w, this.h);
    ctx.textBaseline = 'middle';
    ctx.textAlign = 'left';
    for (var i = 0; i < this.lanes.length; i++) this._drawLane(ctx, this.lanes[i], w);
  },

  _drawLane: function (ctx, lane) {
    var w = this.w, y = lane.y;
    // Every element flows — no fixed labels or track lines, so nothing reads
    // as "stuck". Topic identity comes from the lane colour + event names.
    for (var j = 0; j < lane.packets.length; j++) {
      var p = lane.packets[j], x = p.x;
      if (x > w + 30 || x < -260) continue;
      var fadeIn = Math.min(1, (w - x) / 70);
      var fadeOut = Math.min(1, (x + 30) / 90);
      var a = Math.max(0, Math.min(fadeIn, fadeOut));
      if (a <= 0) continue;

      var sc = p.status === 'ok' ? lane.color : (p.status === 'warn' ? '#FBBF24' : '#FB7185');

      // dot with glow
      ctx.save();
      ctx.shadowColor = sc;
      ctx.shadowBlur = 9 * a;
      ctx.beginPath();
      ctx.arc(x, y, 3.4, 0, Math.PI * 2);
      ctx.fillStyle = this._hexA(sc, 0.95 * a);
      ctx.fill();
      ctx.restore();

      // status glyph for non-ok
      if (p.status !== 'ok') {
        ctx.font = '600 11px "JetBrains Mono", monospace';
        ctx.fillStyle = this._hexA(sc, 0.9 * a);
        ctx.fillText(p.status === 'warn' ? '!' : '×', x - 12, y);
      }

      // event name (lane-coloured — carries the topic identity)
      ctx.font = '500 12px "JetBrains Mono", monospace';
      ctx.fillStyle = this._hexA(lane.color, 0.86 * a);
      ctx.fillText(p.event, x + 12, y);
      var evW = ctx.measureText(p.event).width;

      // partition + offset meta
      ctx.font = '400 10px "JetBrains Mono", monospace';
      ctx.fillStyle = 'rgba(160,180,200,' + (0.5 * a) + ')';
      ctx.fillText('p' + p.partition + ' · ' + p.offset, x + 12 + evW + 10, y);
    }
  },

  stop: function () {
    this.running = false;
    if (this.raf) cancelAnimationFrame(this.raf);
  }
};

// ═══════════════════════════════════════════════════════════
// NAVIGATION
// ═══════════════════════════════════════════════════════════
const OperanNav = {
  init: function() {
    var nav = document.getElementById('nav');
    if (!nav) return;
    window.addEventListener('scroll', function() {
      nav.classList.toggle('scrolled', window.scrollY > 40);
    });
    var toggle = document.getElementById('navToggle');
    var links = document.getElementById('navLinks');
    if (toggle && links) {
      toggle.addEventListener('click', function() {
        links.classList.toggle('open');
        toggle.classList.toggle('active');
      });
      var linkEls = links.querySelectorAll('.nav-link');
      for (var i = 0; i < linkEls.length; i++) {
        linkEls[i].addEventListener('click', function() {
          links.classList.remove('open');
          toggle.classList.remove('active');
        });
      }
    }
  }
};

// ═══════════════════════════════════════════════════════════
// SCROLL REVEAL
// ═══════════════════════════════════════════════════════════
const OperanReveal = {
  init: function() {
    var obs = new IntersectionObserver(function(entries) {
      for (var i = 0; i < entries.length; i++) {
        if (entries[i].isIntersecting) {
          entries[i].target.classList.add('visible');
          obs.unobserve(entries[i].target);
        }
      }
    }, { threshold: 0.1, rootMargin: '0px 0px -40px 0px' });
    document.querySelectorAll('.reveal, .reveal-left, .reveal-right').forEach(function(el) { obs.observe(el); });
  }
};

// ═══════════════════════════════════════════════════════════
// ANIMATED COUNTERS
// ═══════════════════════════════════════════════════════════
const OperanCounters = {
  init: function() {
    var obs = new IntersectionObserver(function(entries) {
      entries.forEach(function(entry) {
        if (entry.isIntersecting && !entry.target.dataset.counted) {
          entry.target.dataset.counted = '1';
          var target = parseInt(entry.target.dataset.target);
          if (isNaN(target)) return;
          var duration = 2000;
          var start = performance.now();
          function animate(now) {
            var progress = Math.min((now - start) / duration, 1);
            var eased = 1 - Math.pow(1 - progress, 3);
            entry.target.textContent = Math.floor(eased * target).toLocaleString();
            if (progress < 1) requestAnimationFrame(animate);
            else entry.target.textContent = target.toLocaleString();
          }
          requestAnimationFrame(animate);
        }
      });
    }, { threshold: 0.5 });
    document.querySelectorAll('[data-target]').forEach(function(el) { obs.observe(el); });
  }
};

// ═══════════════════════════════════════════════════════════
// 3D CARD TILT
// ═══════════════════════════════════════════════════════════
const Operan3DCards = {
  init: function(sel) {
    sel = sel || '.tilt-card';
    document.querySelectorAll(sel).forEach(function(card) {
      card.style.perspective = '1000px';
      card.style.transformStyle = 'preserve-3d';
      card.addEventListener('mousemove', function(e) {
        var rect = card.getBoundingClientRect();
        var x = (e.clientX - rect.left) / rect.width;
        var y = (e.clientY - rect.top) / rect.height;
        var tiltX = (y - 0.5) * -12;
        var tiltY = (x - 0.5) * 12;
        card.style.transform = 'rotateX(' + tiltX + 'deg) rotateY(' + tiltY + 'deg) scale(1.02)';
        card.style.setProperty('--mx', (x*100)+'%');
        card.style.setProperty('--my', (y*100)+'%');
      });
      card.addEventListener('mouseleave', function() {
        card.style.transform = 'rotateX(0) rotateY(0) scale(1)';
      });
    });
  }
};

// ═══════════════════════════════════════════════════════════
// TABS
// ═══════════════════════════════════════════════════════════
const OperanTabs = {
  init: function(groupId) {
    var container = document.getElementById(groupId);
    if (!container) return;
    var btns = container.querySelectorAll('.tab-btn');
    var panels = container.querySelectorAll('.tab-panel');
    for (var i = 0; i < btns.length; i++) {
      btns[i].addEventListener('click', function() {
        var tabId = this.dataset.tab;
        for (var j = 0; j < btns.length; j++) btns[j].classList.remove('active');
        this.classList.add('active');
        for (var k = 0; k < panels.length; k++) panels[k].classList.remove('active');
        var target = container.querySelector('[data-panel="' + tabId + '"]');
        if (target) target.classList.add('active');
      });
    }
  }
};

// ═══════════════════════════════════════════════════════════
// MAGNETIC CURSOR GLOW
// ═══════════════════════════════════════════════════════════
const OperanCursorGlow = {
  init: function() {
    var glow = document.getElementById('cursor-glow');
    if (!glow) return;
    var x = 0, y = 0, cx = 0, cy = 0;
    document.addEventListener('mousemove', function(e) { x = e.clientX; y = e.clientY; });
    (function animate() {
      cx += (x - cx) * 0.12;
      cy += (y - cy) * 0.12;
      glow.style.left = cx + 'px';
      glow.style.top = cy + 'px';
      requestAnimationFrame(animate);
    })();
  }
};

// ═══════════════════════════════════════════════════════════
// SMOOTH SCROLL
// ═══════════════════════════════════════════════════════════
const OperanSmoothScroll = {
  init: function() {
    document.querySelectorAll('a[href^="#"]').forEach(function(a) {
      a.addEventListener('click', function(e) {
        var href = this.getAttribute('href');
        if (href === '#') return;
        var target = document.querySelector(href);
        if (target) { e.preventDefault(); target.scrollIntoView({ behavior: 'smooth', block: 'start' }); }
      });
    });
  }
};

// ═══════════════════════════════════════════════════════════
// FLOATING DOTS
// ═══════════════════════════════════════════════════════════
const OperanFloatingDots = {
  init: function(containerId) {
    var container = document.getElementById(containerId);
    if (!container) return;
    for (var i = 0; i < 15; i++) {
      var d = document.createElement('div');
      d.className = 'floating-dot';
      d.style.left = Math.random()*100+'%';
      d.style.animationDuration = (8+Math.random()*12)+'s';
      d.style.animationDelay = (Math.random()*10)+'s';
      d.style.width = d.style.height = (1+Math.random()*2)+'px';
      container.appendChild(d);
    }
  }
};

// ═══════════════════════════════════════════════════════════
// HERO PARALLAX LAYERS
// ═══════════════════════════════════════════════════════════
const OperanParallax = {
  init: function() {
    var layers = document.querySelectorAll('[data-parallax]');
    if (layers.length === 0) return;
    var ticking = false;
    window.addEventListener('scroll', function() {
      if (!ticking) {
        requestAnimationFrame(function() {
          var scrollY = window.scrollY;
          layers.forEach(function(layer) {
            var speed = parseFloat(layer.dataset.parallax);
            layer.style.transform = 'translateY(' + (scrollY * speed) + 'px)';
          });
          ticking = false;
        });
        ticking = true;
      }
    });
  }
};

// ═══════════════════════════════════════════════════════════
// RAIN ANIMATION (shared, all pages)
// ═══════════════════════════════════════════════════════════
const OperanRain = {
  canvas: null, ctx: null, w: 0, h: 0,
  drops: [], raf: null, active: false,

  init(canvasId) {
    this.canvas = document.getElementById(canvasId);
    if (!this.canvas) return;
    this.ctx = this.canvas.getContext('2d');
    this.active = true;
    this.resize();
    this.initDrops();
    this.animate();
  },

  resize() {
    this.w = this.canvas.width = window.innerWidth;
    this.h = this.canvas.height = window.innerHeight;
    this.initDrops();
  },

  initDrops() {
    var count = Math.floor(this.w / 9);
    this.drops = [];
    for (var i = 0; i < count; i++) {
      this.drops.push({
        x: Math.random() * this.w,
        y: Math.random() * this.h,
        speed: 2 + Math.random() * 6,
        length: 8 + Math.random() * 20,
        opacity: 0.03 + Math.random() * 0.07
      });
    }
  },

  animate() {
    if (!this.active) return;
    this.raf = requestAnimationFrame(this.animate.bind(this));
    this.ctx.clearRect(0, 0, this.w, this.h);
    for (var i = 0; i < this.drops.length; i++) {
      var d = this.drops[i];
      this.ctx.beginPath();
      this.ctx.moveTo(d.x, d.y);
      this.ctx.lineTo(d.x + 0.5, d.y + d.length);
      this.ctx.strokeStyle = 'rgba(100, 160, 200, ' + d.opacity + ')';
      this.ctx.lineWidth = 0.8;
      this.ctx.stroke();
      d.y += d.speed;
      if (d.y > this.h) {
        d.y = -d.length;
        d.x = Math.random() * this.w;
      }
    }
  },

  stop() {
    this.active = false;
    if (this.raf) cancelAnimationFrame(this.raf);
  }
};

// ═══════════════════════════════════════════════════════════
// HERO SLIDESHOW (shared, all pages)
// ═══════════════════════════════════════════════════════════
const OperanSlideshow = {
  container: null, slides: [], dots: [], current: 0, timer: null, fading: false,
  init(containerId, interval) {
    this.container = document.getElementById(containerId);
    if (!this.container) return;
    this.slides = this.container.querySelectorAll('.slide');
    this.dots = document.querySelectorAll('.slide-dot');
    if (this.slides.length === 0) return;
    var iv = interval || 5000;
    this.slides[0].classList.add('active');
    if (this.dots.length) this.dots[0].classList.add('active');
    this.cycle(iv);
  },
  cycle: function(iv) {
    var self = this;
    this.timer = setInterval(function() { self.next(); }, iv);
  },
  next: function() {
    if (this.fading) return;
    this.fading = true;
    var cur = this.slides[this.current];
    if (this.dots[this.current]) this.dots[this.current].classList.remove('active');
    this.current = (this.current + 1) % this.slides.length;
    var nxt = this.slides[this.current];
    cur.classList.remove('active');
    var self = this;
    setTimeout(function() {
      nxt.classList.add('active');
      if (self.dots[self.current]) self.dots[self.current].classList.add('active');
      self.fading = false;
    }, 800);
  },
  pause: function() { clearInterval(this.timer); },
  resume: function(iv) { this.cycle(iv); }
};

// ═══════════════════════════════════════════════════════════
// INIT ON DOM READY
// ═══════════════════════════════════════════════════════════
document.addEventListener('DOMContentLoaded', function() {
  OperanNav.init();
  OperanReveal.init();
  OperanCounters.init();
  Operan3DCards.init('.tilt-card');
  OperanTabs.init('industryTabs');
  OperanCursorGlow.init();
  OperanSmoothScroll.init();
  OperanFloatingDots.init('floating-dots');
  OperanParallax.init();

  // Rain on all pages
  var rainCanvas = document.getElementById('rain-canvas');
  if (rainCanvas) {
    OperanRain.init('rain-canvas');
    rainCanvas.classList.add('active');
  }

  // Real-time event stream (index only, if canvas exists)
  if (document.getElementById('webgl-canvas')) {
    OperanDataVisual.init('webgl-canvas');
  }

  // Slideshow
  if (document.getElementById('hero-slider')) {
    OperanSlideshow.init('hero-slider', 5000);
  }

  window.addEventListener('resize', function() {
    if (OperanRain.canvas) OperanRain.resize();
  });
});