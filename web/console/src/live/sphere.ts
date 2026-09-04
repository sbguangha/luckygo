/**
 * 3D 球体渲染复刻 moshang-ax/lottery（Three.js CSS3D + TWEEN）。
 * 数据源只有 names：轮询接口后调用 updateBall，不改旋转 / 粒子 / 高亮动画。
 */
import * as THREE from "three";
import { CSS3DObject, CSS3DRenderer } from "three/examples/jsm/renderers/CSS3DRenderer.js";
import { TrackballControls } from "three/examples/jsm/controls/TrackballControls.js";
import { Easing, Group, Tween } from "@tweenjs/tween.js";
import confetti from "canvas-confetti";

const ROW_COUNT = 7;
const COLUMN_COUNT = 17;
const TOTAL_CARDS = ROW_COUNT * COLUMN_COUNT;
const COMPANY = "LuckyGo";
const ROTATE_TIME = 3000;
const ROTATE_LOOP = 1000;
const BASE_HEIGHT = 1080;
const PLACEHOLDER = "?";

export class LotterySphere {
  names: string[] = [];
  isLotting = false;

  private container: HTMLElement | null = null;
  private camera: THREE.PerspectiveCamera | null = null;
  private scene: THREE.Scene | null = null;
  private renderer: CSS3DRenderer | null = null;
  private controls: TrackballControls | null = null;
  private threeDCards: CSS3DObject[] = [];
  private targets = { table: [] as THREE.Object3D[], sphere: [] as THREE.Object3D[] };
  private tweenGroup = new Group();
  private rotateTween: Tween<THREE.Euler> | null = null;
  private selectedCardIndex: number[] = [];
  private currentLuckys: string[] = [];
  private running = false;
  private shineTimer = 0;
  private resolution = 1;
  private slotNames: string[] = [];

  initScene(container: HTMLElement) {
    this.destroy();
    this.container = container;
    this.resolution = window.innerHeight / BASE_HEIGHT;
    this.running = true;
    this.slotNames = Array(TOTAL_CARDS).fill(PLACEHOLDER);

    const camera = new THREE.PerspectiveCamera(40, window.innerWidth / window.innerHeight, 1, 10000);
    camera.position.z = 3000;
    this.camera = camera;

    const scene = new THREE.Scene();
    this.scene = scene;

    const position = {
      x: (140 * COLUMN_COUNT - 20) / 2,
      y: (180 * ROW_COUNT - 20) / 2,
    };

    let index = 0;
    for (let i = 0; i < ROW_COUNT; i++) {
      for (let j = 0; j < COLUMN_COUNT; j++) {
        const element = this.createCard(this.nameAt(index), index);
        const object = new CSS3DObject(element);
        object.position.x = Math.random() * 4000 - 2000;
        object.position.y = Math.random() * 4000 - 2000;
        object.position.z = Math.random() * 4000 - 2000;
        scene.add(object);
        this.threeDCards.push(object);

        const tableObj = new THREE.Object3D();
        tableObj.position.x = j * 140 - position.x;
        tableObj.position.y = -(i * 180) + position.y;
        this.targets.table.push(tableObj);
        index++;
      }
    }

    const vector = new THREE.Vector3();
    for (let i = 0, l = this.threeDCards.length; i < l; i++) {
      const phi = Math.acos(-1 + (2 * i) / l);
      const theta = Math.sqrt(l * Math.PI) * phi;
      const object = new THREE.Object3D();
      object.position.setFromSphericalCoords(800 * this.resolution, phi, theta);
      vector.copy(object.position).multiplyScalar(2);
      object.lookAt(vector);
      this.targets.sphere.push(object);
    }

    const renderer = new CSS3DRenderer();
    renderer.setSize(window.innerWidth, window.innerHeight);
    renderer.domElement.style.position = "absolute";
    renderer.domElement.style.inset = "0";
    container.appendChild(renderer.domElement);
    this.renderer = renderer;

    const controls = new TrackballControls(camera, renderer.domElement);
    controls.rotateSpeed = 0.5;
    controls.minDistance = 500;
    controls.maxDistance = 6000;
    controls.addEventListener("change", this.render);
    this.controls = controls;

    window.addEventListener("resize", this.onWindowResize);
    this.transform(this.targets.table, 2000);
    this.animate();
    this.shineCard();
  }

  /** 只替换数据源，不重建场景、不改旋转特效。抽奖旋转中冻结名单，避免没中的人被刷掉。 */
  updateBall(next: string[]) {
    this.names = next.slice();
    if (this.isLotting) return;
    this.syncSlots(this.names);
    this.refreshCardLabels();
  }

  switchToSphere() {
    this.transform(this.targets.sphere, 2000);
  }

  startRotate() {
    const scene = this.scene;
    if (!scene || this.isLotting) return;
    this.clearPrizeCards();
    this.syncSlots(this.names);
    this.refreshCardLabels();
    this.isLotting = true;
    scene.rotation.y = 0;
    this.rotateTween?.stop();
    const tw = new Tween(scene.rotation, this.tweenGroup)
      .to({ y: Math.PI * 6 * ROTATE_LOOP }, ROTATE_TIME * ROTATE_LOOP)
      .onUpdate(this.render);
    this.rotateTween = tw;
    tw.start();
  }

  stopRotate(): Promise<void> {
    return new Promise((resolve) => {
      if (this.rotateTween) {
        this.rotateTween.stop();
        this.rotateTween = null;
      }
      if (this.scene) this.scene.rotation.y = 0;
      this.render();
      resolve();
    });
  }

  highlightWinners(winners: string[]) {
    this.currentLuckys = winners.slice();
    this.selectedCardIndex = [];
    const used = new Set<number>();
    for (const name of winners) {
      this.selectedCardIndex.push(this.cardIndexForName(name, used));
    }
    this.selectCard(600);
    confetti({ particleCount: 160, spread: 70, origin: { y: 0.62 } });
  }

  private cardIndexForName(name: string, used: Set<number>) {
    const hit = this.slotNames.findIndex((n, i) => n === name && !used.has(i));
    if (hit >= 0) {
      used.add(hit);
      return hit;
    }
    const empty = this.slotNames.findIndex((n, i) => n === PLACEHOLDER && !used.has(i));
    if (empty >= 0) {
      used.add(empty);
      return empty;
    }
    for (let i = 0; i < TOTAL_CARDS; i++) {
      if (!used.has(i)) {
        used.add(i);
        return i;
      }
    }
    return 0;
  }

  private clearPrizeCards() {
    for (const i of this.selectedCardIndex) {
      this.threeDCards[i]?.element.classList.remove("prize");
    }
    this.selectedCardIndex = [];
    this.currentLuckys = [];
  }

  destroy() {
    this.running = false;
    window.clearInterval(this.shineTimer);
    window.removeEventListener("resize", this.onWindowResize);
    this.tweenGroup.removeAll();
    this.rotateTween?.stop();
    this.controls?.dispose();
    if (this.renderer?.domElement.parentElement) {
      this.renderer.domElement.parentElement.removeChild(this.renderer.domElement);
    }
    this.threeDCards = [];
    this.targets = { table: [], sphere: [] };
    this.camera = null;
    this.scene = null;
    this.renderer = null;
    this.controls = null;
    this.container = null;
    this.isLotting = false;
    this.slotNames = [];
  }

  private nameAt(i: number) {
    return this.slotNames[i] || PLACEHOLDER;
  }

  /** 一人一格、按加入顺序占空位；已占格不挪动，离开的人留下问号。 */
  private syncSlots(incoming: string[]) {
    if (this.slotNames.length !== TOTAL_CARDS) {
      this.slotNames = Array(TOTAL_CARDS).fill(PLACEHOLDER);
    }
    const want = incoming.map((n) => n.trim()).filter(Boolean);
    const used = new Array(want.length).fill(false);
    const next = Array(TOTAL_CARDS).fill(PLACEHOLDER);
    for (let i = 0; i < TOTAL_CARDS; i++) {
      const cur = this.slotNames[i];
      if (cur === PLACEHOLDER) continue;
      const found = want.findIndex((n, j) => !used[j] && n === cur);
      if (found >= 0) {
        used[found] = true;
        next[i] = cur;
      }
    }
    for (let j = 0; j < want.length; j++) {
      if (used[j]) continue;
      const idx = next.indexOf(PLACEHOLDER);
      if (idx < 0) break;
      next[idx] = want[j];
    }
    this.slotNames = next;
  }

  private refreshCardLabels() {
    this.threeDCards.forEach((obj, i) => {
      if (this.selectedCardIndex.includes(i)) return;
      this.changeCard(i, this.nameAt(i));
    });
  }

  private createCard(name: string, id: number) {
    const element = document.createElement("div");
    element.id = "card-" + id;
    element.className = "element";
    element.style.backgroundColor = "rgba(0,127,127," + (Math.random() * 0.7 + 0.25) + ")";
    const company = document.createElement("div");
    company.className = "company";
    company.textContent = COMPANY;
    const nameEl = document.createElement("div");
    nameEl.className = "name";
    nameEl.textContent = name;
    const details = document.createElement("div");
    details.className = "details";
    details.textContent = "年会互动";
    element.append(company, nameEl, details);
    return element;
  }

  private changeCard(cardIndex: number, name: string) {
    const card = this.threeDCards[cardIndex]?.element;
    if (!card) return;
    const nameEl = card.querySelector(".name");
    if (nameEl) nameEl.textContent = name;
  }

  private transform(targets: THREE.Object3D[], duration: number) {
    for (let i = 0; i < this.threeDCards.length; i++) {
      const object = this.threeDCards[i];
      const target = targets[i];
      new Tween(object.position, this.tweenGroup)
        .to({ x: target.position.x, y: target.position.y, z: target.position.z }, Math.random() * duration + duration)
        .easing(Easing.Exponential.InOut)
        .start();
      new Tween(object.rotation, this.tweenGroup)
        .to({ x: target.rotation.x, y: target.rotation.y, z: target.rotation.z }, Math.random() * duration + duration)
        .easing(Easing.Exponential.InOut)
        .start();
    }
    new Tween({ t: 0 }, this.tweenGroup).to({ t: 1 }, duration * 2).onUpdate(this.render).start();
  }

  private selectCard(duration = 600) {
    const width = 140;
    const locates: { x: number; y: number }[] = [];
    const l = this.selectedCardIndex.length;
    if (l > 5) {
      const yPosition = [-87, 87];
      const mid = Math.ceil(l / 2);
      let tag = -(mid - 1) / 2;
      for (let i = 0; i < mid; i++) {
        locates.push({ x: tag * width * this.resolution, y: yPosition[0] * this.resolution });
        tag++;
      }
      tag = -(l - mid - 1) / 2;
      for (let i = mid; i < l; i++) {
        locates.push({ x: tag * width * this.resolution, y: yPosition[1] * this.resolution });
        tag++;
      }
    } else {
      let tag = -(l - 1) / 2;
      for (let i = 0; i < l; i++) {
        locates.push({ x: tag * width * this.resolution, y: 0 });
        tag++;
      }
    }

    this.selectedCardIndex.forEach((cardIndex, index) => {
      this.changeCard(cardIndex, this.currentLuckys[index] || PLACEHOLDER);
      const object = this.threeDCards[cardIndex];
      new Tween(object.position, this.tweenGroup)
        .to({ x: locates[index].x, y: locates[index].y, z: 2200 }, Math.random() * duration + duration)
        .easing(Easing.Exponential.InOut)
        .start();
      new Tween(object.rotation, this.tweenGroup)
        .to({ x: 0, y: 0, z: 0 }, Math.random() * duration + duration)
        .easing(Easing.Exponential.InOut)
        .start();
      object.element.classList.add("prize");
    });

    new Tween({ t: 0 }, this.tweenGroup)
      .to({ t: 1 }, duration * 2)
      .onUpdate(this.render)
      .onComplete(() => {
        this.isLotting = false;
      })
      .start();
  }

  private shineCard() {
    window.clearInterval(this.shineTimer);
    this.shineTimer = window.setInterval(() => {
      if (this.isLotting) return;
      const shineN = 10 + Math.floor(Math.random() * 10);
      for (let i = 0; i < shineN; i++) {
        const cardIndex = Math.floor(Math.random() * TOTAL_CARDS);
        if (this.selectedCardIndex.includes(cardIndex)) continue;
        const card = this.threeDCards[cardIndex]?.element;
        if (card) {
          card.style.backgroundColor = "rgba(0,127,127," + (Math.random() * 0.7 + 0.25) + ")";
        }
      }
    }, 500);
  }

  private onWindowResize = () => {
    if (!this.camera || !this.renderer) return;
    this.camera.aspect = window.innerWidth / window.innerHeight;
    this.camera.updateProjectionMatrix();
    this.renderer.setSize(window.innerWidth, window.innerHeight);
    this.render();
  };

  private animate = () => {
    if (!this.running) return;
    requestAnimationFrame(this.animate);
    this.tweenGroup.update();
    this.controls?.update();
  };

  private render = () => {
    if (this.renderer && this.scene && this.camera) {
      this.renderer.render(this.scene, this.camera);
    }
  };
}
