import { ComponentType } from 'react';
import SlidingWindow from './SlidingWindow';
import SequenceModel from './SequenceModel';
import Boosting from './Boosting';
import RLLoop from './RLLoop';
import Voting from './Voting';
import AlgoPredict from './AlgoPredict';

export interface VizEntry {
  component: ComponentType<{ playing: boolean; arg?: string }>;
  defaultCaption: string;
  /** true = component fetches live data from the API; false = purely illustrative */
  dataDriven?: boolean;
  /**
   * true = the component manages its own play/pause controls internally.
   * VizBlock will hide its outer Play/Stop button for these entries.
   */
  ownControls?: boolean;
}

const registry: Record<string, VizEntry> = {
  'sliding-window': {
    component: SlidingWindow,
    defaultCaption: 'Cửa sổ trượt tính trung bình động (MA) — từng bước cộng dồn trên số thật',
    dataDriven: true,
    ownControls: true,
  },
  'sequence-model': {
    component: SequenceModel,
    defaultCaption: 'LSTM/GRU: chuỗi nạp tuần tự vào ô nhớ',
    dataDriven: false,
  },
  boosting: {
    component: Boosting,
    defaultCaption: 'Gradient Boosting: mỗi cây sửa phần sai còn lại',
    dataDriven: false,
  },
  'rl-loop': {
    component: RLLoop,
    defaultCaption: 'Reinforcement Learning: vòng lặp quan sát - hành động - thưởng',
    dataDriven: false,
  },
  voting: {
    component: Voting,
    defaultCaption: 'Ensemble: bỏ phiếu có trọng số từng bước — trace số thật',
    dataDriven: true,
    ownControls: true,
  },
  'algo-predict': {
    component: AlgoPredict,
    defaultCaption: 'Dự đoán thật của thuật toán trên dữ liệu hiện tại',
    dataDriven: true,
    ownControls: true,
  },
};

export default registry;
