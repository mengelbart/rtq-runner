#!/usr/bin/env python
import os
import json
import pandas as pd
import matplotlib.pyplot as plt
import matplotlib.dates as mdates
import argparse

from glob import glob
from matplotlib.ticker import EngFormatter
from jinja2 import Environment, FileSystemLoader

class rates_plot:
    def __init__(self, name):
        self.name = name
        self.labels = []
        self.fig, self.ax = plt.subplots(figsize=(8,2), dpi=400)

    def add_rtp(self, file, basetime, label):
        if not os.path.exists(file):
            return False
        df = pd.read_csv(
                file,
                index_col = 0,
                names = ['time', 'rate'],
                header = None,
                usecols = [0, 6],
            )
        df.index = pd.to_datetime(df.index - basetime, unit='ms')
        df['rate'] = df['rate'].apply(lambda x: x * 8)
        df = df.resample('1s').sum()
        l, = self.ax.plot(df.index, df.values, label=label, linewidth=0.5)
        self.labels.append(l)
        return True

    def add_cc(self, file, basetime):
        if not os.path.exists(file):
            return False
        df = pd.read_csv(
                file,
                index_col = 0,
                names = ['time', 'target'],
                header = None,
                usecols = [0, 1],
            )
        df.index = pd.to_datetime(df.index - basetime, unit='ms')
        df = df[df['target'] > 0]
        l, = self.ax.plot(df.index, df.values, label='Target Bitrate', linewidth=0.5)
        self.labels.append(l)
        return True

    def add_router(self, file, basetime):
        if not os.path.exists(file):
            return False

        df = pd.read_csv(
                file,
                index_col = 0,
                names = ['time', 'bandwidth'],
                header = None,
                usecols = [0, 1],
            )
        df.index = pd.to_datetime(df.index - basetime, unit='ms')
        l, = self.ax.step(df.index, df.values, where='post', label='Capacity', linewidth=0.5)
        self.labels.append(l)
        return True

    def plot(self, path):
        plt.xlabel('time')
        plt.ylabel('rate')
        self.ax.legend(handles=self.labels)
        self.ax.xaxis.set_major_formatter(mdates.DateFormatter("%M:%S"))
        self.ax.yaxis.set_major_formatter(EngFormatter(unit='bit/s'))

        plt.savefig(os.path.join(path, self.name + '.png'))

class qlog_cwnd_plot:
    def __init__(self, name):
        self.name = name
        self.fig, self.ax = plt.subplots(figsize=(8,2), dpi=400)
        self.labels = []

    def add_cwnd(self, file):
        if not os.path.exists(file):
            return False

        congestion = []
        with open(file) as f:
            for index, line in enumerate(f):
                event = json.loads(line.strip())
                if 'name' in event and event['name'] == 'recovery:metrics_updated':
                    if 'data' in event and 'congestion_window' in event['data']:
                        congestion.append({'time': event['time'], 'cwnd': event['data']['congestion_window']})

        df = pd.DataFrame(congestion)
        df.index = pd.to_datetime(df['time'], unit='ms')
        l, = self.ax.plot(df.index, df['cwnd'], label='CWND')
        self.labels.append(l)
        return True

    def plot(self, path):
        plt.xlabel('Time')
        plt.ylabel('CWND')

        self.ax.legend(handles=self.labels)
        self.ax.xaxis.set_major_formatter(mdates.DateFormatter("%M:%S"))
        self.ax.yaxis.set_major_formatter(EngFormatter(unit='Bytes'))

        plt.savefig(os.path.join(path, self.name + '.png'))

class qlog_bytes_in_flight_plot:
    def __init__(self, name):
        self.name = name
        self.fig, self.ax = plt.subplots(figsize=(9,4), dpi=400)
        self.labels = []

    def add_bytes_in_flight(self, file):
        if not os.path.exists(file):
            return False

        inflight = []
        dgram = []
        stream = []
        sums = []
        with open(file) as f:
            for index, line in enumerate(f):
                event = json.loads(line.strip())
                if 'name' in event and event['name'] == 'recovery:metrics_updated':
                    if 'data' in event and 'bytes_in_flight' in event['data']:
                        inflight.append({'time': event['time'], 'bytes_in_flight': event['data']['bytes_in_flight']})
                if 'name' in event and event['name'] == 'transport:packet_sent':
                    if 'data' in event and 'frames' in event['data']:
                        datagrams = [frame for frame in event['data']['frames'] if frame['frame_type'] == 'datagram' ]
                        stream_frames = [frame for frame in event['data']['frames'] if frame['frame_type'] == 'stream' ]
                        if len(datagrams) > 0:
                            dgram.append({'time': event['time'], 'bytes': sum([datagram['length'] for datagram in datagrams])})
                            sums.append({'time': event['time'], 'bytes': sum([datagram['length'] for datagram in datagrams])})
                        if len(stream_frames) > 0:
                            stream.append({'time': event['time'], 'bytes': sum([stream['length'] for stream in stream_frames])})
                            sums.append({'time': event['time'], 'bytes': sum([stream['length'] for stream in stream_frames])})


        df = pd.DataFrame(inflight)
        df.index = pd.to_datetime(df['time'], unit='ms')

        datagram_df = pd.DataFrame(dgram)
        datagram_df.index = pd.to_datetime(datagram_df['time'], unit='ms')
        datagram_df = datagram_df.resample('1s').sum()

        stream_df = pd.DataFrame(stream)
        stream_df.index = pd.to_datetime(stream_df['time'], unit='ms')
        stream_df = stream_df.resample('1s').sum()

        sums_df = pd.DataFrame(sums)
        sums_df.index = pd.to_datetime(sums_df['time'], unit='ms')
        sums_df = sums_df.resample('1s').sum()


        l0, = self.ax.plot(df.index, df['bytes_in_flight'], label='Bytes in Flight')
        l1, = self.ax.plot(datagram_df.index, datagram_df['bytes'], label='Datagram Bytes Sent')
        l2, = self.ax.plot(stream_df.index, stream_df['bytes'], label='Stream Bytes Sent')
        l3, = self.ax.plot(sums_df.index, sums_df['bytes'], label='Total sent')

        self.labels.append(l0)
        self.labels.append(l1)
        self.labels.append(l2)
        self.labels.append(l3)
        return True

    def plot(self, path):
        plt.xlabel('Time')
        plt.ylabel('Bytes in Flight')

        self.ax.legend(handles=self.labels)
        self.ax.xaxis.set_major_formatter(mdates.DateFormatter("%M:%S"))
        self.ax.yaxis.set_major_formatter(EngFormatter(unit='Bytes'))

        plt.savefig(os.path.join(path, self.name + '.png'))


class qlog_rtt_plot:
    def __init__(self, name):
        self.name = name
        self.fig, self.ax = plt.subplots(figsize=(8,2), dpi=400)
        self.labels = []

    def add_rtt(self, file):
        if not os.path.exists(file):
            return False

        rtt = []
        with open(file) as f:
            for index, line in enumerate(f):
                event = json.loads(line.strip())
                if 'name' in event and event['name'] == 'recovery:metrics_updated':
                    append = False
                    sample = {'time': event['time']}
                    if 'data' in event and 'smoothed_rtt' in event['data']:
                        sample['smoothed_rtt'] = event['data']['smoothed_rtt']
                        append = True
                    if 'data' in event and 'min_rtt' in event['data']:
                        sample['min_rtt'] = event['data']['min_rtt']
                        append = True
                    if 'data' in event and 'latest_rtt' in event['data']:
                        sample['latest_rtt'] = event['data']['latest_rtt']
                        append = True
                    if append:
                        rtt.append(sample)

        df = pd.DataFrame(rtt)
        df.index = pd.to_datetime(df['time'], unit='ms')
        l, = self.ax.plot(df.index, df['latest_rtt'], label='Latest RTT')
        self.labels.append(l)
        return True

    def plot(self, path):
        plt.xlabel('Time')
        plt.ylabel('RTT')

        self.ax.legend(handles=self.labels)
        self.ax.xaxis.set_major_formatter(mdates.DateFormatter("%M:%S"))
        self.ax.yaxis.set_major_formatter(EngFormatter(unit='ms'))

        plt.savefig(os.path.join(path, self.name + '.png'))

class tcp_plot:
    def __init__(self, name):
        self.name = name
        self.fig, self.ax = plt.subplots(figsize=(8,2), dpi=400)
        self.labels = []

    def add_router(self, file, basetime):
        if not os.path.exists(file):
            return False

        df = pd.read_csv(
                file,
                index_col = 0,
                names = ['time', 'bandwidth'],
                header = None,
                usecols = [0, 1],
            )
        df.index = pd.to_datetime(df.index - basetime, unit='ms')
        l, = self.ax.step(df.index, df.values, where='post', label='Bandwidth', linewidth=0.5)
        self.labels.append(l)
        return True

    def add(self, file, label):
        if not os.path.exists(file):
            return False

        with open(file) as data_file:
            data = json.load(data_file)

        df = pd.json_normalize(data, record_path='intervals')
        df.index = pd.to_datetime(df['sum.start'], unit='s')
        df = df.resample('1s').mean()

        l, = self.ax.plot(df.index, df['sum.bits_per_second'], label=label, linewidth=0.5)
        self.labels.append(l)
        return True

    def plot(self, path):
        plt.xlabel('Time')
        plt.ylabel('Rate')

        self.ax.legend(handles=self.labels)
        self.ax.xaxis.set_major_formatter(mdates.DateFormatter("%M:%S"))
        self.ax.yaxis.set_major_formatter(EngFormatter(unit='bit/s'))

        plt.savefig(os.path.join(path, self.name + '-plot.png'))

def generate_html(path):
    images = [os.path.basename(x) for x in glob(os.path.join(path, '*.png'))]
    print(images)
    templates_dir = './templates/'
    env = Environment(loader = FileSystemLoader(templates_dir))
    template = env.get_template('index.html')

    filename = os.path.join(path, 'index.html')
    with open(filename, 'w') as fh:
        fh.write(template.render(images = images))

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("plot")

    parser.add_argument("--input_dir")
    parser.add_argument("--output_dir")
    parser.add_argument("--basetime", type=int, default=0)
    parser.add_argument("--router")

    args = parser.parse_args()

    match args.plot:
        case 'rates':
            basetime = pd.to_datetime(args.basetime, unit='s').timestamp() * 1000
            plot = rates_plot(args.plot)
            plot.add_rtp(os.path.join(args.input_dir, 'send_log', 'rtp_out.log'), basetime, 'RTP sent')
            plot.add_rtp(os.path.join(args.input_dir, 'receive_log', 'rtp_in.log'), basetime, 'RTP received')
            plot.add_cc(os.path.join(args.input_dir, 'send_log', 'gcc.log'), basetime)
            plot.add_cc(os.path.join(args.input_dir, 'send_log', 'scream.log'), basetime)
            plot.add_router(os.path.join(args.input_dir, args.router), basetime)
            plot.plot(args.output_dir)

        case 'qlog-cwnd':
            qlog_files = glob(os.path.join(args.input_dir, 'send_log', '*.qlog'))
            basetime = pd.to_datetime(args.basetime, unit='s').timestamp() * 1000
            plot = qlog_cwnd_plot(args.plot)
            if len(qlog_files) > 0:
                if plot.add_cwnd(qlog_files[0]):
                    plot.plot(args.output_dir)

        case 'qlog-in-flight':
            qlog_files = glob(os.path.join(args.input_dir, 'send_log', '*.qlog'))
            basetime = pd.to_datetime(args.basetime, unit='s').timestamp() * 1000
            plot = qlog_bytes_in_flight_plot(args.plot)
            if len(qlog_files) > 0:
                if plot.add_bytes_in_flight(qlog_files[0]):
                    plot.plot(args.output_dir)

        case 'qlog-rtt':
            qlog_files = glob(os.path.join(args.input_dir, 'send_log', '*.qlog'))
            basetime = pd.to_datetime(args.basetime, unit='s').timestamp() * 1000
            plot = qlog_rtt_plot(args.plot)
            if len(qlog_files) > 0:
                if plot.add_rtt(qlog_files[0]):
                    plot.plot(args.output_dir)

        case 'html':
            generate_html(args.output_dir)

if __name__ == "__main__":
    main()


