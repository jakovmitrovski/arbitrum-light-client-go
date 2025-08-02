import numpy as np
import matplotlib.pyplot as plt
import pandas as pd
from typing import Dict, List, Tuple, Optional
from pathlib import Path

import glob
import csv
import os
import json
import tqdm

from cycler import cycler
import matplotlib
import matplotlib.pyplot as plt
import matplotlib.lines as mlines
from matplotlib.patches import Patch
import seaborn as sns

TUMRed = '#cf0000'
TUMDarkRed = '#990000'
TUMBlue = '#0091ff'
TUMDarkerBlue = '#005fc6'
TUMOrange = '#ff590d'
TUMGreen = '#A3AD00'
TUMDarkerGreen = '#52cc00'
TUMDarkGreen = '#00cc00'
TUMVeryDarkGreen = '#008000'
TUMDarkYellow = '#ffb300'

def set_size(width, fraction=1, subplots=(1, 1)):
    """Set figure dimensions to avoid scaling in LaTeX.

    Parameters
    ----------
    width: float or string
            Document width in points, or string of predined document type
    fraction: float, optional
            Fraction of the width which you wish the figure to occupy
    subplots: array-like, optional
            The number of rows and columns of subplots.
    Returns
    -------
    fig_dim: tuple
            Dimensions of figure in inches
    """
    if width == 'single':
        width_pt = 252.0
    elif width == 'double':
        width_pt = 516.0
    else:
        width_pt = width

    # Width of figure (in pts)
    fig_width_pt = width_pt * fraction
    # Convert from pt to inches
    inches_per_pt = 1 / 72.27

    # Golden ratio to set aesthetic figure height
    # https://disq.us/p/2940ij3
    golden_ratio = (5**.5 - 1) / 2

    # Figure width in inches
    fig_width_in = fig_width_pt * inches_per_pt
    # Figure height in inches
    fig_height_in = fig_width_in * golden_ratio * (subplots[0] / subplots[1])

    return (fig_width_in, fig_height_in)

# oracle
# FONT_SIZE = 32
# tournament
FONT_SIZE = 24
# use it before plotting
def rc_setting():

    # matplotlib.use("pgf")

    #Direct input

    linestyle_cycler = (cycler("color", plt.cm.viridis(np.linspace(0,1,5))) + cycler('linestyle',['-','--',':','-.', '-']))
    
    #Options
    return {
        # Use LaTeX to write all text
        "text.usetex": True,
 
        "font.family": "serif",
        "font.serif": ["Computer Modern Roman"],
        # "font.monospace": ["Computer Modern Typewriter"],

        # "font.serif": ["Computer Modern Roman"],
        # "font.monospace": ["Computer Modern Typewriter"],
        # Use 10pt font in plots, to match 10pt font in document
        "axes.labelsize": FONT_SIZE,
        "font.size": FONT_SIZE,
        "font.weight": "bold",
        # Make the legend/label fonts a little smaller
        "legend.fontsize": 20,
        "xtick.labelsize": FONT_SIZE,
        "ytick.labelsize": FONT_SIZE,
        #"axes.prop_cycle":linestyle_cycler,
        "axes.grid": False,
        "grid.linestyle": "--",
        "grid.linewidth": 0.5,  # default 0.8
        "grid.color": "c0c0c0c0", # default #b0b0b0
        "axes.axisbelow": True,
        "legend.framealpha": 1,
        "patch.linewidth": 0.8, # default 1.0
        "legend.handlelength": 1.0, # default 2.0
        "legend.handletextpad": 0.5, # default 0.8
        "legend.columnspacing": 0.8, # default 2
    }

plt.rcParams.update(rc_setting())


# rc('text', usetex=True)
# rc(
#     'font',
#     family='serif',
#     serif=['Computer Modern Roman'],
#     monospace=['Computer Modern Typewriter'],
#     size=FONT_SIZE
# )

def load_csv_data(file_path: str) -> pd.DataFrame:
    """Load CSV data from measurement files"""
    try:
        df = pd.read_csv(file_path)
        return df
    except FileNotFoundError:
        print(f"Warning: Could not find file {file_path}")
        return pd.DataFrame()

def calculate_stats_by_provers(df: pd.DataFrame, metric: str) -> Dict[int, Tuple[float, float, List[float]]]:
    """Calculate mean, std, and all values for each number of provers"""
    stats = {}
    
    if 'num_provers' not in df.columns:
        # For consensus/execution data without num_provers column
        if len(df) > 0:
            mean_val = df[metric].mean()
            std_val = df[metric].std()
            all_values = df[metric].tolist()
            stats[1] = (mean_val, std_val, all_values)
        return stats
    
    # Handle data with num_provers column
    for num_provers in sorted(df['num_provers'].unique()):
        subset = df[df['num_provers'] == num_provers]
        if len(subset) > 0:
            mean_val = subset[metric].mean()
            std_val = subset[metric].std()
            all_values = subset[metric].tolist()
            stats[num_provers] = (mean_val, std_val, all_values)
    
    return stats

def plot_metric_with_std(df: pd.DataFrame, metric: str, title: str, ylabel: str, 
                        colors: List[str] = None, save_path: str = None):
    """Plot a metric with standard deviation for different numbers of provers"""
    if len(df) == 0:
        print(f"No data available for {metric}")
        return

    
    stats = calculate_stats_by_provers(df, metric)
    if not stats:
        print(f"No valid data for {metric}")
        return
    
    plt.figure(figsize=(12, 8))
    
    if 'num_provers' in df.columns and len(df['num_provers'].unique()) > 1:
        provers_list = sorted(stats.keys())
        x_positions = np.arange(len(provers_list))
        
        means = [stats[p][0] for p in provers_list]
        stds = [stats[p][1] for p in provers_list]
        
        # Use different colors for multiple provers
        colors = ['#1f77b4', '#ff7f0e', '#2ca02c', '#d62728', '#9467bd', '#8c564b']
        for i, (prover_count, mean_val, std_val) in enumerate(zip(provers_list, means, stds)):
            display_provers = prover_count + 1
            plt.errorbar([i], [mean_val], yerr=[std_val], fmt='o-', 
                        capsize=5, capthick=2, linewidth=2, markersize=8,
                        color=colors[i % len(colors)], label=f'{display_provers} provers')
        
        # Display prover numbers as 1, 2, 3... instead of 0, 1, 2...
        display_provers = [p + 1 for p in provers_list]
        plt.xticks(x_positions, [f'{p} provers' for p in display_provers])
        plt.xlabel('Number of Provers')
        
    else:
        # For consensus/execution data or single prover tournament data - use blue
        mean_val, std_val, all_values = stats[list(stats.keys())[0]]
        plt.errorbar([0], [mean_val], yerr=[std_val], fmt='o-',
                    capsize=5, capthick=2, linewidth=2, markersize=8,
                    color='#1f77b4', label=f'{metric.replace("_", " ").title()}')
        plt.xticks([0], ['Single Measurement'])
        plt.xlabel('Measurement Type')
    
    plt.ylabel(ylabel)
    plt.grid(True, alpha=0.3)
    plt.legend()
    
    if save_path:
        plt.savefig(save_path, bbox_inches='tight', dpi=300, pad_inches=0.2)
        print(f"Saved plot to {save_path}")
    
    #plt.show()

def plot_all_metrics_for_type(data_type: str, df: pd.DataFrame, output_dir: str):
    """Plot all metrics for a specific data type (consensus, execution, or tournament)"""
    if len(df) == 0:
        print(f"No data available for {data_type}")
        return
    
    # Create output directory
    os.makedirs(output_dir, exist_ok=True)
    
    # Define metrics to plot based on data type
    if data_type == 'tournament':
        # For tournament, skip individual plots - we only want block-based plots
        pass
    else:  # consensus or execution - will be handled in main function
        pass

def plot_combined_oracle_metrics(consensus_df: pd.DataFrame, execution_df: pd.DataFrame, output_dir: str):
    """Create separate plots for consensus and execution oracles"""
    
    # Define metrics to plot (without oracle type prefix)
    metrics = [
        ('oracle_time_ms', 'Execution Time (s)', 'oracle'),
        ('cpu_percent', 'CPU Usage (percent)', 'cpu'),
        ('memory_bytes', 'Memory Usage (MB)', 'memory'),
        ('network_bytes_in', 'Inbound Traffic (MB)', 'network_in'),
        ('network_bytes_out', 'Outbound Traffic (MB)', 'network_out')
    ]
    
    # Plot consensus oracle metrics
    if len(consensus_df) > 0:
        print("Creating consensus oracle plots...")
        plot_oracle_metrics(consensus_df, "Consensus", metrics, output_dir, "consensus")
    
    # Plot execution oracle metrics
    if len(execution_df) > 0:
        print("Creating execution oracle plots...")
        plot_oracle_metrics(execution_df, "Execution", metrics, output_dir, "execution")

def plot_oracle_metrics(df: pd.DataFrame, oracle_type: str, metrics: list, output_dir: str, filename_prefix: str):
    """Create individual plots for each metric of a specific oracle type"""
    
    for metric, title, metric_type in metrics:
        # Handle oracle time with prefix, other metrics without prefix
        if metric == 'oracle_time_ms':
            column_name = f'{oracle_type.lower()}_oracle_time_ms'
        else:
            column_name = metric
            
        if column_name in df.columns:
            # Group by block number and calculate statistics
            oracle_stats = {}
            for block_num in sorted(df['block_number'].unique()):
                block_data = df[df['block_number'] == block_num]
                values = block_data[column_name].values
                
                # Convert units for better readability
                if metric == 'oracle_time_ms':
                    values = values / 1000  # Convert to seconds
                elif metric == 'memory_bytes':
                    values = values / (1024 * 1024)  # Convert to MB
                elif metric in ['network_bytes_in', 'network_bytes_out']:
                    values = values / (1024 * 1024)  # Convert to MB
                
                mean_val = np.mean(values)
                std_val = np.std(values)
                oracle_stats[block_num] = (mean_val, std_val)
            
            # Plot oracle data
            oracle_blocks = sorted(oracle_stats.keys())
            oracle_means = [oracle_stats[b][0] for b in oracle_blocks]
            oracle_stds = [oracle_stats[b][1] for b in oracle_blocks]
            
            # Create individual figure for this metric
            plt.figure(figsize=(12, 8))
            
            # Plot with shaded standard deviation
            plt.fill_between(oracle_blocks, 
                          [m - s for m, s in zip(oracle_means, oracle_stds)],
                          [m + s for m, s in zip(oracle_means, oracle_stds)],
                          alpha=0.3, color='#1f77b4')
            
            # Plot mean line
            plt.plot(oracle_blocks, oracle_means, 'o-', color='#1f77b4', 
                   linewidth=2, markersize=6, label=oracle_type)
            
            plt.xlabel(r'Block Number')
            
            # Set proper y-axis labels based on metric type
            if metric == 'oracle_time_ms':
                ylabel = r'Execution Time (s)'
            elif metric == 'cpu_percent':
                ylabel = r'CPU Usage (\%)'
            elif metric == 'memory_bytes':
                ylabel = r'Memory Usage (MB)'
            elif metric == 'network_bytes_in':
                ylabel = r'Inbound Traffic (MB)'
            elif metric == 'network_bytes_out':
                ylabel = r'Outbound Traffic (MB)'
            else:
                ylabel = title.split('(')[1].split(')')[0] if '(' in title else 'Value'
            
            plt.ylabel(ylabel)
            plt.grid(True, alpha=0.3)
            plt.legend()
            
            # Set appropriate Y-axis range based on metric type
            if 'CPU Usage' in title:
                # CPU percentage: 0-100%
                plt.ylim(0, 100)
            elif 'Memory Usage' in title:
                # Memory in MB: 0-15 GB (15000 MB)
                plt.ylim(0, 15000)
            elif 'Inbound Traffic' in title:
                # Network in MB: 0-50 MB
                plt.ylim(0, 50)
            elif 'Outbound Traffic' in title:
                # Network out in MB: 0-50 MB
                plt.ylim(0, 50)
            elif 'Execution Time' in title:
                # Time in seconds: 0-30 seconds
                plt.ylim(0, 30)
            
            # Recursive dynamic Y-axis adjustment for low values
            all_values = []
            if column_name in df.columns:
                oracle_values = df[column_name].values
                if metric == 'oracle_time_ms':
                    oracle_values = oracle_values / 1000
                elif metric == 'memory_bytes':
                    oracle_values = oracle_values / (1024 * 1024)
                elif metric in ['network_bytes_in', 'network_bytes_out']:
                    oracle_values = oracle_values / (1024 * 1024)
                all_values.extend(oracle_values)
            
            if len(all_values) > 0:
                data_min = np.min(all_values)
                data_max = np.max(all_values)
                data_range = data_max - data_min
                
                # Include some padding for better visualization
                data_range_with_padding = data_range * 1.2  # Add 20% padding
                
                # Keep zooming until we get a reasonable data-to-range ratio
                max_iterations = 3  # Reduce from 5 to 3
                iteration = 0
                current_ylim = plt.ylim()
                
                while iteration < max_iterations:
                    current_range = current_ylim[1] - current_ylim[0]
                    
                    # Check if data range is still too small compared to current plot range
                    if data_range_with_padding < current_range * 0.3:  # Less than 30% of current range (increased from 15%)
                        center = (data_min + data_max) / 2
                        # Use a larger portion of the current range for next zoom
                        expanded_range = current_range * 0.5  # Use 50% of current range (increased from 30%)
                        y_min_dynamic = max(0, center - expanded_range / 2)
                        y_max_dynamic = min(current_ylim[1], center + expanded_range / 2)
                        plt.ylim(y_min_dynamic, y_max_dynamic)
                        current_ylim = (y_min_dynamic, y_max_dynamic)
                        iteration += 1
                    else:
                        break  # Data is now reasonably visible
            

            
            plt.tight_layout()
            
            # Save individual plot
            metric_name = metric.replace('_', '_')
            save_path = os.path.join(output_dir, f'{filename_prefix}_{metric_name}.pdf')
            plt.savefig(save_path, bbox_inches='tight', dpi=300, pad_inches=0.2)
            print(f"Saved {oracle_type.lower()} {metric} plot to {save_path}")
            plt.close()
            
            print(f"Plotted {oracle_type.lower()} {metric} with {len(oracle_means)} averaged values")
        else:
            print(f"{oracle_type} {column_name} not found in columns: {list(df.columns)}")

def plot_combined_tournament(df: pd.DataFrame, output_dir: str):
    """Create combined tournament plots showing metrics vs block numbers for each prover count"""
    if len(df) == 0 or 'num_provers' not in df.columns:
        print("No tournament data available")
        return
    
    # Create final combined plots for all metrics (up to 10K blocks)
    final_metrics = [
        ('sync_time_ms', r'Tournament Sync Time vs Block Number - All Prover Configurations', r'Sync Time (minutes)', 0, 30),
        ('cpu_percent', r'Tournament CPU Usage vs Block Number - All Prover Configurations', r'CPU Usage (percent)', 0, 100),
        ('memory_bytes', r'Tournament Memory Usage vs Block Number - All Prover Configurations', r'Memory Usage (GB)', 0, 15),
        ('network_bytes_in', r'Tournament Inbound Traffic vs Block Number - All Prover Configurations', r'Inbound Traffic (MB)', 0, 50),
        ('network_bytes_out', r'Tournament Outbound Traffic vs Block Number - All Prover Configurations', r'Outbound Traffic (MB)', 0, 50)
    ]
    
    for metric, title, ylabel, y_min, y_max in final_metrics:
        if metric in df.columns:
            plot_final_metric_comparison(df, metric, title, ylabel, y_min, y_max, output_dir, max_blocks=10100)
            plot_final_metric_comparison_extended(df, metric, title, ylabel, y_min, y_max, output_dir)

def plot_final_metric_comparison(df: pd.DataFrame, metric: str, title: str, ylabel: str, y_min: float, y_max: float, output_dir: str, max_blocks: int = 10100):
    """Create final plot showing a specific metric vs block numbers for all prover counts (up to max_blocks)"""
    if len(df) == 0:
        return

    
    plt.figure(figsize=(12, 8))
    
    colors = ['#1f77b4', '#ff7f0e', '#2ca02c', '#d62728', '#9467bd', '#8c564b']
    unique_provers = sorted(df['num_provers'].unique())
    
    all_means = []  # Collect all means to determine dynamic y-axis range
    
    for i, prover_count in enumerate(unique_provers):
        display_provers = prover_count + 1
        subset = df[df['num_provers'] == prover_count]
        
        if len(subset) == 0:
            continue
        
        # Group by block number and calculate statistics
        block_stats = {}
        for block_num in sorted(subset['block_number'].unique()):
            if block_num > max_blocks:
                continue
            block_data = subset[subset['block_number'] == block_num]
            values = block_data[metric].values
            
            # Convert units for better readability
            if metric == 'sync_time_ms':
                values = values / (1000 * 60)  # Convert to minutes
            elif metric == 'memory_bytes':
                values = values / (1024 * 1024 * 1024)  # Convert to GB
            elif metric in ['network_bytes_in', 'network_bytes_out']:
                values = values / (1024 * 1024)  # Convert to MB
            
            mean_val = np.mean(values)
            std_val = np.std(values)
            block_stats[block_num] = (mean_val, std_val)
        
        # Plot data
        block_numbers = sorted(block_stats.keys())
        means = [block_stats[b][0] for b in block_numbers]
        stds = [block_stats[b][1] for b in block_numbers]
        
        all_means.extend(means)  # Collect for y-axis calculation
        
        # Plot with shaded standard deviation
        plt.fill_between(block_numbers, 
                        [m - s for m, s in zip(means, stds)],
                        [m + s for m, s in zip(means, stds)],
                        alpha=0.2, color=colors[i % len(colors)])
        
        # Plot mean line
        plt.plot(block_numbers, means, 'o-', 
               color=colors[i % len(colors)], linewidth=3, markersize=8,
               label=f'{display_provers} provers')
    
    plt.xlabel(r'Block Number')
    plt.ylabel(ylabel)
    
    # Set x-axis to linear scale and limit to max_blocks
    plt.xlim(100, max_blocks)
    
    # Add full node reference line only for sync time
    if metric == 'sync_time_ms':
        # For sync time, use the full node comparison line
        full_node_x, full_node_y = create_full_node_comparison_line([0, max_blocks], start_block=0, end_block=max_blocks)
        if full_node_x is not None and full_node_y is not None:
            plt.plot(full_node_x, full_node_y, 'k--', linewidth=2, alpha=0.7, label='Full Node')

         
    # Recursive dynamic y-axis range based on actual data
    if all_means:
        data_min = min(all_means)
        data_max = max(all_means)
        data_range = data_max - data_min
        
        # Include standard deviation in the range calculation
        if len(stds) > 0:
            max_std = max(stds)
            data_min_with_std = data_min - max_std
            data_max_with_std = data_max + max_std
            data_range_with_std = data_max_with_std - data_min_with_std
        else:
            data_range_with_std = data_range
        
        # Keep zooming until we get a reasonable data-to-range ratio
        max_iterations = 3  # Reduce from 5 to 3
        iteration = 0
        current_y_min, current_y_max = y_min, y_max
        
        while iteration < max_iterations:
            current_range = current_y_max - current_y_min
            
            # Check if data range is still too small compared to current plot range
            if data_range_with_std < current_range * 0.3:  # Less than 30% of current range (increased from 15%)
                center = (data_min + data_max) / 2
                # Use a larger portion of the current range for next zoom
                expanded_range = current_range * 0.5  # Use 50% of current range (increased from 30%)
                y_min_dynamic = max(0, center - expanded_range / 2)
                y_max_dynamic = min(y_max, center + expanded_range / 2)
                current_y_min, current_y_max = y_min_dynamic, y_max_dynamic
                iteration += 1
            else:
                break  # Data is now reasonably visible
        
        plt.ylim(current_y_min, current_y_max)
    else:
        plt.ylim(y_min, y_max)
    
    plt.grid(True, alpha=0.3)
    plt.legend()
    
    # Create filename from metric name
    metric_name = metric.replace('_', '_')
    save_path = os.path.join(output_dir, f'tournament_final_{metric_name}_comparison_10k.pdf')
    plt.savefig(save_path, bbox_inches='tight', dpi=300, pad_inches=0.2)
    print(f"Saved final {metric} comparison plot (10K) to {save_path}")
    #plt.show()

def plot_final_metric_comparison_extended(df: pd.DataFrame, metric: str, title: str, ylabel: str, y_min: float, y_max: float, output_dir: str):
    """Create extended final plot showing a specific metric vs block numbers for all prover counts (up to 100M)"""
    if len(df) == 0:
        return
    
     
    plt.figure(figsize=(12, 8))
    
    colors = ['#1f77b4', '#ff7f0e', '#2ca02c', '#d62728', '#9467bd', '#8c564b']
    unique_provers = sorted(df['num_provers'].unique())
    
    all_means = []  # Collect all means to determine dynamic y-axis range
    
    for i, prover_count in enumerate(unique_provers):
        display_provers = prover_count + 1
        subset = df[df['num_provers'] == prover_count]
        
        if len(subset) == 0:
            continue
        
        # Group by block number and calculate statistics
        block_stats = {}
        for block_num in sorted(subset['block_number'].unique()):
            block_data = subset[subset['block_number'] == block_num]
            values = block_data[metric].values
            
            # Convert units for better readability
            if metric == 'sync_time_ms':
                values = values / (1000 * 60)  # Convert to minutes
            elif metric == 'memory_bytes':
                values = values / (1024 * 1024 * 1024)  # Convert to GB
            elif metric in ['network_bytes_in', 'network_bytes_out']:
                values = values / (1024 * 1024)  # Convert to MB
            
            mean_val = np.mean(values)
            std_val = np.std(values)
            block_stats[block_num] = (mean_val, std_val)
        
        # Plot data
        block_numbers = sorted(block_stats.keys())
        means = [block_stats[b][0] for b in block_numbers]
        stds = [block_stats[b][1] for b in block_numbers]
        
        all_means.extend(means)  # Collect for y-axis calculation
        
        # Plot with shaded standard deviation
        plt.fill_between(block_numbers, 
                        [m - s for m, s in zip(means, stds)],
                        [m + s for m, s in zip(means, stds)],
                        alpha=0.2, color=colors[i % len(colors)])
        
        # Plot mean line
        plt.plot(block_numbers, means, 'o-', 
               color=colors[i % len(colors)], linewidth=3, markersize=8,
               label=f'{display_provers} provers')
        
        # Add interpolation lines for each prover configuration
        if len(means) > 1:
            if metric == 'sync_time_ms':
                # Logarithmic interpolation for sync time
                interp_x, interp_y = create_logarithmic_interpolation(block_numbers, means, False)
            elif metric in ['network_bytes_in', 'network_bytes_out']:
                # Logarithmic interpolation for network metrics
                interp_x, interp_y = create_logarithmic_interpolation(block_numbers, means, True)
            else:
                # Constant interpolation for other metrics (CPU, memory)
                interp_x, interp_y = create_constant_interpolation(block_numbers, means)
            
            if interp_x is not None and interp_y is not None:
                # Use same color as the data line, no legend
                plt.plot(interp_x, interp_y, '--', color=colors[i % len(colors)], 
                        linewidth=2, alpha=0.6)
    
    plt.xlabel(r'Block Number')
    plt.ylabel(ylabel)
    
    # Set x-axis to logarithmic scale and extend to 100 million
    plt.xscale('log')
    plt.xlim(100, 100000000)
    
    # Add red vertical line at block 10100
    plt.axvline(x=10100, color='red', linestyle='--', linewidth=2, alpha=0.7, label=r'Block 10100')
    
    # Add full node reference line only for sync time
    if metric == 'sync_time_ms':
        # For sync time, use the full node comparison line
        # Use proper range for extended plot: 0 to 1 million blocks
        full_node_x, full_node_y = create_full_node_comparison_line([], start_block=0, end_block=1000000)
        if full_node_x is not None and full_node_y is not None:
            plt.plot(full_node_x, full_node_y, 'k--', linewidth=2, alpha=0.7, label=r'Full Node')
        # Set Y-axis to 25 minutes for sync time (extended version)
        plt.ylim(0, 25)
    else:
        # Recursive dynamic y-axis range based on actual data
        if all_means:
            data_min = min(all_means)
            data_max = max(all_means)
            data_range = data_max - data_min
            
            # Include standard deviation in the range calculation
            if len(stds) > 0:
                max_std = max(stds)
                data_min_with_std = data_min - max_std
                data_max_with_std = data_max + max_std
                data_range_with_std = data_max_with_std - data_min_with_std
            else:
                data_range_with_std = data_range
            
            # Keep zooming until we get a reasonable data-to-range ratio
            max_iterations = 3  # Reduce from 5 to 3
            iteration = 0
            current_y_min, current_y_max = y_min, y_max
            
            while iteration < max_iterations:
                current_range = current_y_max - current_y_min
                
                # Check if data range is still too small compared to current plot range
                if data_range_with_std < current_range * 0.3:  # Less than 30% of current range (increased from 15%)
                    center = (data_min + data_max) / 2
                    # Use a larger portion of the current range for next zoom
                    expanded_range = current_range * 0.5  # Use 50% of current range (increased from 30%)
                    y_min_dynamic = max(0, center - expanded_range / 2)
                    y_max_dynamic = min(y_max, center + expanded_range / 2)
                    current_y_min, current_y_max = y_min_dynamic, y_max_dynamic
                    iteration += 1
                else:
                    break  # Data is now reasonably visible
            
            plt.ylim(current_y_min, current_y_max)
        else:
            plt.ylim(y_min, y_max)
    
    plt.grid(True, alpha=0.3)
    plt.legend()
    
    # Create filename from metric name
    metric_name = metric.replace('_', '_')
    save_path = os.path.join(output_dir, f'tournament_final_{metric_name}_comparison_extended.pdf')
    plt.savefig(save_path, bbox_inches='tight', dpi=300, pad_inches=0.2)
    print(f"Saved extended final {metric} comparison plot to {save_path}")
    #plt.show()



def create_trend_line(block_numbers, values, start_block=10000, end_block=100000):
    """Create a trend line that follows the data from start_block to end_block"""
    # Filter data points between start_block and end_block
    trend_data = [(b, v) for b, v in zip(block_numbers, values) if start_block <= b <= end_block]
    
    if len(trend_data) < 2:
        return None, None
    
    # Extract x and y values for trend calculation
    trend_x = [b for b, v in trend_data]
    trend_y = [v for b, v in trend_data]
    
    # Calculate trend using linear regression on log scale
    if len(trend_x) > 1:
        # Use logarithmic trend for better fit
        log_x = np.log10(trend_x)
        coeffs = np.polyfit(log_x, trend_y, 1)
        
        # Create trend line from start_block to end_block (100k)
        trend_line_x = np.logspace(np.log10(start_block), np.log10(end_block), 100)
        trend_line_y = coeffs[0] * np.log10(trend_line_x) + coeffs[1]
        return trend_line_x, trend_line_y
    
    return None, None

def create_linear_comparison_line(block_numbers, start_block=10000, end_block=100000000):
    """Create a linear comparison line for sync time"""
    # Create a linear line that grows proportionally to block number
    # Starting from a reasonable baseline at start_block
    baseline_time = 0.5  # 0.5 minutes at start_block
    growth_rate = 0.0005  # Growth per block
    
    linear_x = np.linspace(start_block, end_block, 100)
    linear_y = baseline_time + growth_rate * (linear_x - start_block)
    
    return linear_x, linear_y

def create_linear_interpolation(block_numbers, values, start_block=4000, end_block=100000000):
    """Create linear interpolation from start_block onwards"""
    # Filter data points from start_block onwards
    interpolation_data = [(b, v) for b, v in zip(block_numbers, values) if b >= start_block]
    
    if len(interpolation_data) < 2:
        return None, None
    
    interp_x = [b for b, v in interpolation_data]
    interp_y = [v for b, v in interpolation_data]
    
    # Calculate linear trend
    coeffs = np.polyfit(interp_x, interp_y, 1)
    
    # Create interpolation line from start_block to end_block
    interp_line_x = np.linspace(start_block, end_block, 1000)
    interp_line_y = coeffs[0] * interp_line_x + coeffs[1]
    
    return interp_line_x, interp_line_y

def create_logarithmic_interpolation(block_numbers, values, all, start_block=4100, end_block=100000000):
    """Create logarithmic interpolation from start_block onwards for sync time"""
    # Filter data points from start_block onwards
    if not all:
        interpolation_data = [(b, v) for b, v in zip(block_numbers, values) if b >= start_block and b <= 8100]
    else:
        interpolation_data = [(b, v) for b, v in zip(block_numbers, values) if b >= 100]
    
    if len(interpolation_data) < 2:
        return None, None
    
    # Extract x and y values for interpolation - use all available data
    interp_x = [b for b, v in interpolation_data]
    interp_y = [v for b, v in interpolation_data]

    # Calculate logarithmic trend
    log_x = np.log10(interp_x)
    coeffs = np.polyfit(log_x, interp_y, 1)
    
    # Create interpolation line from start_block to end_block
    interp_line_x = np.logspace(np.log10(start_block), np.log10(end_block), 1000)
    interp_line_y = coeffs[0] * np.log10(interp_line_x) + coeffs[1]
    
    return interp_line_x, interp_line_y

def create_constant_interpolation(block_numbers, values, start_block=4000, end_block=100000000):
    """Create constant interpolation from start_block onwards"""
    # Filter data points from start_block onwards
    interpolation_data = [(b, v) for b, v in zip(block_numbers, values) if b >= start_block]
    
    if len(interpolation_data) < 1:
        return None, None
    
    # Calculate the average value from start_block onwards
    interp_y_values = [v for b, v in interpolation_data]
    constant_value = np.mean(interp_y_values)
    
    # Create constant line from start_block to end_block
    interp_line_x = np.linspace(start_block, end_block, 1000)
    interp_line_y = np.full_like(interp_line_x, constant_value)
    
    return interp_line_x, interp_line_y

def create_exponential_comparison_line(block_numbers, start_block=10000, end_block=1000000):
    """Create an exponential comparison line for sync time"""
    # Create an exponential line that grows exponentially with block number
    # Starting from a reasonable baseline at start_block
    baseline_time = 0.5  # 0.5 minutes at start_block
    growth_factor = 1.000001  # Very small exponential growth factor per block
    
    exponential_x = np.linspace(start_block, end_block, 1000)
    exponential_y = baseline_time * (growth_factor ** (exponential_x - start_block))
    
    return exponential_x, exponential_y

def create_full_node_comparison_line(block_numbers, start_block=0, end_block=100000000):
    """Create a full node comparison line for sync time following (15 * x) / (1000 * 60)"""
    # Create a line that follows the formula (15 * x) / (1000 * 60)
    # This represents 15ms per block converted to minutes
    
    full_node_x = np.linspace(start_block, end_block, 1000)
    # Fix the formula: ensure proper order of operations
    full_node_y = (15.0 * full_node_x) / (1000.0 * 60.0)  # 15ms per block converted to minutes
    
    return full_node_x, full_node_y

def print_statistics(df: pd.DataFrame, data_type: str):
    """Print detailed statistics for the data"""
    if len(df) == 0:
        print(f"No data available for {data_type}")
        return
    
    print(f"\n=== {data_type.upper()} STATISTICS ===")
    
    if 'num_provers' in df.columns:
        unique_provers = df['num_provers'].unique()
        print(f"Number of different prover configurations: {len(unique_provers)}")
        for num_provers in sorted(unique_provers):
            subset = df[df['num_provers'] == num_provers]
            print(f"\n{num_provers} provers (n={len(subset)}):")
            for col in df.columns:
                if col in ['num_provers', 'block_number', 'iteration', 'timestamp']:
                    continue
                if col in subset.columns:
                    mean_val = subset[col].mean()
                    std_val = subset[col].std()
                    print(f"  {col}: {mean_val:.2f} ± {std_val:.2f}")
    else:
        print(f"Total measurements: {len(df)}")
        for col in df.columns:
            if col in ['block_number', 'iteration', 'timestamp']:
                continue
            if col in df.columns:
                mean_val = df[col].mean()
                std_val = df[col].std()
                print(f"  {col}: {mean_val:.2f} ± {std_val:.2f}")

def main():
    # Define data directories
    base_dir = Path(".")
    consensus_file = base_dir / "consensus" / "consensus_oracle_measurements.csv"
    execution_file = base_dir / "execution" / "execution_oracle_measurements.csv"
    tournament_file = base_dir / "tournament" / "tournament_measurements_real.csv"
    
    # Load data
    print("Loading measurement data...")
    consensus_df = load_csv_data(consensus_file)
    execution_df = load_csv_data(execution_file)
    tournament_df = load_csv_data(tournament_file)
    
    # Create output directories
    output_dirs = {
        'tournament': base_dir / "plots" / "tournament",
        'oracles': base_dir / "plots" / "oracles"
    }
    
    for dir_path in output_dirs.values():
        dir_path.mkdir(parents=True, exist_ok=True)
    
    # Print statistics
    print_statistics(consensus_df, "consensus")
    print_statistics(execution_df, "execution")
    print_statistics(tournament_df, "tournament")
    
    # Generate plots
    print("\nGenerating plots...")
    
    # Combined oracle plots
    print("Creating combined oracle plots...")
    plot_combined_oracle_metrics(consensus_df, execution_df, output_dirs['oracles'])
    
    # Tournament plots (block-based only)
    print("Creating tournament plots...")
    plot_combined_tournament(tournament_df, output_dirs['tournament'])
    
    print("\nAll plots generated successfully!")
    print(f"Check the following directories for plots:")
    for data_type, dir_path in output_dirs.items():
        print(f"  {data_type}: {dir_path}")

if __name__ == "__main__":
    main()